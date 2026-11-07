/*
|--------------------------------------------------------------------------
| Notification Recipients
|--------------------------------------------------------------------------
|
| A notification goes to one workspace member: the bell's rows are theirs,
| mail goes to their user's address, pushes to the browsers they enabled,
| and live updates to their own realtime channel. The member is named by
| workspace and member together, so a queued delivery can load them again
| inside the right workspace. A member who left, or whose user is gone,
| is simply no longer there to notify.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RecipientType names workspace members as notification recipients.
*/
const RecipientType = "workspace_member"

/*
Recipient is a member a notification reaches, with their user's
address, name, language and time zone, and the browsers they enabled
push on.
*/
type Recipient struct {
	WorkspaceID   ids.UUID
	MemberID      ids.UUID
	UserID        ids.UUID
	Role          string
	Email         string
	Name          string
	Language      string
	Timezone      string
	EmailVerified bool
	Push          []webpush.Subscription
}

/*
NotifiableType names the recipient type.
*/
func (Recipient) NotifiableType() string { return RecipientType }

/*
NotifiableID names the member inside their workspace.
*/
func (r Recipient) NotifiableID() string { return RecipientKey(r.WorkspaceID, r.MemberID) }

/*
Locale is the language the member's user picked.
*/
func (r Recipient) Locale() string { return r.Language }

/*
RouteFor returns where each channel reaches the member: a verified
address for mail, their browsers for push, their own channel for live
updates.
*/
func (r Recipient) RouteFor(channel string) any {
	switch channel {
	case "mail":
		if !r.EmailVerified || r.Email == "" {
			return nil
		}
		return r.Email
	case "webpush":
		return r.Push
	case "broadcast":
		return appbroadcasting.MemberChannel(r.MemberID)
	}
	return nil
}

/*
RecipientKey names a member inside a workspace as notifications store
it.
*/
func RecipientKey(workspaceID, memberID ids.UUID) string {
	return workspaceID.String() + "/" + memberID.String()
}

/*
parseKey reads a key back into the workspace and member.
*/
func parseKey(key string) (ids.UUID, ids.UUID, error) {
	ws, member, ok := strings.Cut(key, "/")
	if !ok {
		return ids.UUID{}, ids.UUID{}, fnotify.ErrNotifiableGone
	}
	w, err := ids.Parse(ws)
	if err != nil {
		return ids.UUID{}, ids.UUID{}, fnotify.ErrNotifiableGone
	}
	m, err := ids.Parse(member)
	if err != nil {
		return ids.UUID{}, ids.UUID{}, fnotify.ErrNotifiableGone
	}
	return w, m, nil
}

/*
Recipients loads members as recipients.
*/
type Recipients struct {
	DB   *database.DB
	Keys *encryption.Keys
}

/*
Resolve loads the member a key names, for a queued delivery.
*/
func (r *Recipients) Resolve(ctx context.Context, key string) (fnotify.Notifiable, error) {
	ws, member, err := parseKey(key)
	if err != nil {
		return nil, err
	}
	var out []Recipient
	err = r.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		out, err = r.Load(ctx, models.New(tx), ws, []ids.UUID{member})
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fnotify.ErrNotifiableGone
	}
	return out[0], nil
}

/*
Load returns the active members among the ids, every active member when
none are given, with their push subscriptions.
*/
func (r *Recipients) Load(ctx context.Context, q *models.Queries, ws ids.UUID, memberIDs []ids.UUID,
) ([]Recipient, error) {
	if memberIDs == nil {
		memberIDs = []ids.UUID{}
	}
	rows, err := q.ListNotificationRecipients(ctx, models.ListNotificationRecipientsParams{
		WorkspaceID: ws, Ids: memberIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("load recipients: %w", err)
	}
	out := make([]Recipient, 0, len(rows))
	for _, row := range rows {
		subs, err := r.subscriptions(ctx, q, row.UserID)
		if err != nil {
			return nil, err
		}
		out = append(out, Recipient{
			WorkspaceID: row.WorkspaceID, MemberID: row.ID, UserID: row.UserID, Role: row.Role, Email: row.Email,
			Name: row.Name, Language: deref(row.Locale), Timezone: deref(row.Timezone),
			EmailVerified: row.EmailVerifiedAt != nil, Push: subs,
		})
	}
	return out, nil
}

/*
subscriptions returns the user's browsers, their auth secrets opened.
One that no longer opens is skipped; the browser subscribes again.
*/
func (r *Recipients) subscriptions(ctx context.Context, q *models.Queries, userID ids.UUID,
) ([]webpush.Subscription, error) {
	rows, err := q.ListUserPushSubscriptions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load push subscriptions: %w", err)
	}
	out := make([]webpush.Subscription, 0, len(rows))
	for _, row := range rows {
		auth, err := encryption.Open[string](ctx, r.Keys, row.AuthSecretEncrypted, pushAAD(row.EndpointHash))
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		if err != nil {
			continue
		}
		out = append(out, webpush.Subscription{
			Endpoint: row.Endpoint, Keys: webpush.Keys{Auth: auth, P256dh: row.P256dh},
		})
	}
	return out, nil
}

/*
pushAAD binds a sealed auth secret to its endpoint.
*/
func pushAAD(endpointHash []byte) []byte {
	return append([]byte("push_subscriptions.auth_secret_encrypted:"), endpointHash...)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
