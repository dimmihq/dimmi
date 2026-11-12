/*
|--------------------------------------------------------------------------
| One-Click Unsubscribe
|--------------------------------------------------------------------------
|
| Every teammate email carries a signed link that turns that kind of
| email off without signing in, as RFC 8058 asks: a mail app posts to
| it, and a person who opens it lands on a small page that says what
| stopped, offers it back, and points to the full settings. The link
| names the workspace, the member, the trigger and when it stops
| working, and is signed the way broadcast links are, so nobody can
| turn off someone else's mail by guessing. A link for every trigger,
| such as the quiet hours summary carries, turns all of them off.
|
*/

package notifications

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AllTriggers is the trigger of a link that turns every email off.
*/
const AllTriggers = "all"

/*
unsubscribeLife is how long an unsubscribe link keeps working.
*/
const unsubscribeLife = 90 * 24 * time.Hour

/*
ErrBadUnsubscribe refuses a link this install did not sign, or one
that expired.
*/
var ErrBadUnsubscribe = errors.New("notifications: unsubscribe link not recognized")

/*
UnsubscribeRef is what an unsubscribe link names.
*/
type UnsubscribeRef struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	Trigger     string
	Expires     time.Time
}

/*
SignUnsubscribe writes the token of a link: the workspace, member and
trigger, then when it expires in base 36, then the signature over both.
*/
func SignUnsubscribe(key []byte, ref UnsubscribeRef) string {
	raw := make([]byte, 0, 32+len(ref.Trigger))
	raw = append(raw, ref.WorkspaceID[:]...)
	raw = append(raw, ref.MemberID[:]...)
	raw = append(raw, ref.Trigger...)
	payload := base64.RawURLEncoding.EncodeToString(raw) + "." + strconv.FormatInt(ref.Expires.Unix(), 36)
	return payload + "." + unsubscribeSignature(key, payload)
}

/*
ReadUnsubscribe checks a token's signature and expiry and returns what
it names.
*/
func ReadUnsubscribe(key []byte, token string, now time.Time) (UnsubscribeRef, error) {
	cut := strings.LastIndexByte(token, '.')
	if cut < 0 || len(key) == 0 {
		return UnsubscribeRef{}, ErrBadUnsubscribe
	}
	payload, sig := token[:cut], token[cut+1:]
	if !hashing.Equal([]byte(sig), []byte(unsubscribeSignature(key, payload))) {
		return UnsubscribeRef{}, ErrBadUnsubscribe
	}
	encoded, expires, _ := strings.Cut(payload, ".")
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	unix, perr := strconv.ParseInt(expires, 36, 64)
	if err != nil || perr != nil || len(raw) < 33 {
		return UnsubscribeRef{}, ErrBadUnsubscribe
	}
	ref := UnsubscribeRef{Trigger: string(raw[32:]), Expires: time.Unix(unix, 0)}
	copy(ref.WorkspaceID[:], raw[:16])
	copy(ref.MemberID[:], raw[16:32])
	if !now.Before(ref.Expires) {
		return UnsubscribeRef{}, ErrBadUnsubscribe
	}
	return ref, nil
}

/*
unsubscribeSignature signs a link, shortened to keep URLs readable.
*/
func unsubscribeSignature(key []byte, payload string) string {
	return base64.RawURLEncoding.EncodeToString(hashing.HMAC(key, []byte("ntf-unsub:"+payload))[:16])
}

/*
Unsubscribe returns where a mail app posts to turn the trigger's email
off for the member, "" when links are not signed on this install.
*/
func (l Links) Unsubscribe(ws, member ids.UUID, trigger string, now time.Time) string {
	if len(l.Key) == 0 {
		return ""
	}
	token := SignUnsubscribe(l.Key, UnsubscribeRef{
		WorkspaceID: ws, MemberID: member, Trigger: trigger, Expires: now.Add(unsubscribeLife),
	})
	return strings.TrimRight(l.Base, "/") + "/n/u/" + token
}

/*
Unsubscribes turns email off and back on from signed links.
*/
type Unsubscribes struct {
	DB    *database.DB
	Clock clock.Clock
	Key   []byte
	Links Links
}

/*
UnsubscribeState is what the page shows: whose mail, which kind, and
whether it is now on.
*/
type UnsubscribeState struct {
	Workspace   string
	Email       string
	Trigger     string
	Label       string
	Subscribed  bool
	SettingsURL string
}

/*
Show reads what the link names.
*/
func (u *Unsubscribes) Show(ctx context.Context, token string) (UnsubscribeState, error) {
	return u.change(ctx, token, nil)
}

/*
Unsubscribe turns the link's email off.
*/
func (u *Unsubscribes) Unsubscribe(ctx context.Context, token string) (UnsubscribeState, error) {
	off := false
	return u.change(ctx, token, &off)
}

/*
Resubscribe turns the link's email back on.
*/
func (u *Unsubscribes) Resubscribe(ctx context.Context, token string) (UnsubscribeState, error) {
	on := true
	return u.change(ctx, token, &on)
}

/*
change reads the link, sets its email cells when told to, and returns
the state.
*/
func (u *Unsubscribes) change(ctx context.Context, token string, enabled *bool) (UnsubscribeState, error) {
	ref, err := ReadUnsubscribe(u.Key, token, u.Clock.Now())
	if err != nil {
		return UnsubscribeState{}, exceptions.ErrNotFound
	}
	triggers, label, ok := linkTriggers(ref.Trigger)
	if !ok {
		return UnsubscribeState{}, exceptions.ErrNotFound
	}
	var out UnsubscribeState
	scoped := tenancy.With(ctx, ref.WorkspaceID.String())
	err = u.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		who, ok, err := loadMemberOf(ctx, q, ref)
		if err != nil || !ok {
			return err
		}
		if enabled != nil {
			if err := u.setEmail(ctx, q, emailChange{ref: ref, triggers: triggers, enabled: *enabled}); err != nil {
				return err
			}
		}
		out, err = u.state(ctx, q, ref, who)
		out.Label = label
		return err
	})
	if err == nil && out.Workspace == "" {
		err = exceptions.ErrNotFound
	}
	return out, err
}

/*
state reads whether the link's email is on now.
*/
func (u *Unsubscribes) state(ctx context.Context, q *models.Queries, ref UnsubscribeRef, who linkMember,
) (UnsubscribeState, error) {
	choices, err := LoadChoices(ctx, q, ref.WorkspaceID, ref.MemberID)
	if err != nil {
		return UnsubscribeState{}, err
	}
	triggers, _, _ := linkTriggers(ref.Trigger)
	out := UnsubscribeState{
		Workspace: who.workspace.Name, Email: who.user.Email, Trigger: ref.Trigger,
		SettingsURL: u.Links.Settings(who.workspace.Slug),
	}
	for _, t := range triggers {
		out.Subscribed = out.Subscribed || choices.Enabled(t, ChannelEmail)
	}
	return out, nil
}

/*
linkMember is the member a link names, with their user and workspace.
*/
type linkMember struct {
	workspace models.Workspace
	user      models.User
}

/*
loadMemberOf loads the member a link names, reporting false when they
left or are gone.
*/
func loadMemberOf(ctx context.Context, q *models.Queries, ref UnsubscribeRef) (linkMember, bool, error) {
	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: ref.WorkspaceID, ID: ref.MemberID,
	})
	if err != nil {
		return linkMember{}, false, ignoreMissing(err, "load member")
	}
	if member.Status != "active" {
		return linkMember{}, false, nil
	}
	w, err := q.FindWorkspace(ctx, ref.WorkspaceID)
	if err != nil {
		return linkMember{}, false, ignoreMissing(err, "load workspace")
	}
	user, err := q.FindUser(ctx, member.UserID)
	if err != nil {
		return linkMember{}, false, ignoreMissing(err, "load user")
	}
	return linkMember{workspace: w, user: user}, true, nil
}

/*
emailChange turns the email of the link's triggers on or off.
*/
type emailChange struct {
	ref      UnsubscribeRef
	triggers []Trigger
	enabled  bool
}

/*
setEmail turns the email cell of every trigger on or off, forgetting a
cell set back to its default. A locked cell stays on.
*/
func (u *Unsubscribes) setEmail(ctx context.Context, q *models.Queries, c emailChange) error {
	ref, enabled := c.ref, c.enabled
	for _, t := range c.triggers {
		if t.IsLocked(ChannelEmail) {
			continue
		}
		var err error
		if enabled == t.Default(ChannelEmail) {
			err = q.DeleteNotificationPreference(ctx, models.DeleteNotificationPreferenceParams{
				WorkspaceID: ref.WorkspaceID, WorkspaceMemberID: ref.MemberID, Trigger: t.Key, Channel: ChannelEmail,
			})
		} else {
			err = q.UpsertNotificationPreference(ctx, models.UpsertNotificationPreferenceParams{
				WorkspaceID: ref.WorkspaceID, WorkspaceMemberID: ref.MemberID, Trigger: t.Key, Channel: ChannelEmail,
				Enabled: enabled, Now: u.Clock.Now(),
			})
		}
		if err != nil {
			return fmt.Errorf("set email for %s: %w", t.Key, err)
		}
	}
	return nil
}

/*
linkTriggers are the triggers a link names and how the page calls them.
*/
func linkTriggers(key string) ([]Trigger, string, bool) {
	if key == AllTriggers {
		return Triggers(), "all notification emails", true
	}
	t, ok := FindTrigger(key)
	if !ok {
		return nil, "", false
	}
	return []Trigger{t}, t.Label, true
}
