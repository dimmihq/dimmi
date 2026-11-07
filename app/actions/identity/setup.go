/*
|--------------------------------------------------------------------------
| Sign-In Mode
|--------------------------------------------------------------------------
|
| How strict a workspace is about who its messenger visitors are. In
| optional, the default, a signed-in customer is proven by token and
| everyone else chats anonymously. In required, only proven customers
| get a messenger at all. Off ignores tokens entirely, which never means
| trusting what a page types about its visitor.
|
| Switching to required can lock every customer out if the host's
| identity endpoint is broken, so it is refused until the counts show
| at least 100 session starts in the last week of which 95% were signed
| in, unless the owner forces it knowing that. Every widget of the
| workspace is forgotten on a change, so messengers learn the new mode
| on their next page load.
|
*/

package identity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The sign-in modes.
*/
const (
	ModeOff      = "off"
	ModeOptional = "optional"
	ModeRequired = "required"
)

/*
The rollout guard: required needs this many starts over this many days,
this share of them signed in. MaxClientWritable caps the attributes a
signed-in messenger may write itself.
*/
const (
	GuardStarts       = 100
	GuardDays         = 7
	GuardShare        = 0.95
	MaxClientWritable = 20
)

/*
WidgetForgetter makes every replica forget a workspace's widgets.
*/
type WidgetForgetter interface {
	/*
		ForgetAll drops the workspace's widgets from every replica.
	*/
	ForgetAll(ctx context.Context, workspaceID ids.UUID) error
}

/*
Setup reads and changes a workspace's sign-in settings.
*/
type Setup struct {
	DB      *database.DB
	Clock   clock.Clock
	Widgets WidgetForgetter
}

/*
SetupChange is a change to the settings; nil leaves a field as it is.
Force switches to required past the rollout guard.
*/
type SetupChange struct {
	Membership     authz.Membership
	Mode           *string
	Force          bool
	ClientWritable *[]string
}

/*
guardRefusal refuses switching to required, with the counts that fell
short.
*/
func guardRefusal(summary Summary) *fault.Error {
	return fault.New(fault.Conflict, "rollout_guard",
		fmt.Sprintf("Only %.0f%% of %d messenger starts in the last 7 days were signed in; required needs %d "+
			"starts and %.0f%%. Fix your identity endpoint first, or force the switch.",
			summary.Share()*100, summary.Starts(), GuardStarts, GuardShare*100)).
		With("verified", summary.Verified).With("invalid", summary.Invalid).
		With("anonymous", summary.Anon).With("required_percent", int(GuardShare*100))
}

/*
ErrRequiredWithoutSecret refuses required when nothing can verify a
token.
*/
var ErrRequiredWithoutSecret = fault.New(fault.Conflict, "identity_not_configured",
	"Create an identity secret before making sign-in required.")

/*
LoadSettings returns the workspace's settings, the defaults when it
never changed them.
*/
func LoadSettings(ctx context.Context, q *models.Queries, ws ids.UUID) (models.IdentitySetting, error) {
	row, err := q.FindIdentitySettings(ctx, ws)
	if errors.Is(err, database.ErrNotFound) {
		return models.IdentitySetting{WorkspaceID: ws, Mode: ModeOptional, ClientWritableAttributes: []string{}}, nil
	}
	if err != nil {
		return row, fmt.Errorf("load identity settings: %w", err)
	}
	return row, nil
}

/*
Show returns the settings.
*/
func (s *Setup) Show(ctx context.Context, m authz.Membership) (models.IdentitySetting, error) {
	if err := RequireRead(m); err != nil {
		return models.IdentitySetting{}, err
	}
	var out models.IdentitySetting
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = LoadSettings(ctx, models.New(tx), m.Workspace.ID)
		return err
	})
	return out, err
}

/*
Update changes the settings and makes every messenger of the workspace
learn them.
*/
func (s *Setup) Update(ctx context.Context, in SetupChange) (models.IdentitySetting, error) {
	if err := RequireManage(in.Membership); err != nil {
		return models.IdentitySetting{}, err
	}
	ws := in.Membership.Workspace.ID
	var out models.IdentitySetting
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := LoadSettings(ctx, q, ws)
		if err != nil {
			return err
		}
		next, err := s.apply(ctx, q, current, in)
		if err != nil {
			return err
		}
		out, err = q.UpsertIdentitySettings(ctx, models.UpsertIdentitySettingsParams{
			WorkspaceID: ws, Mode: next.Mode, ClientWritableAttributes: next.ClientWritableAttributes,
			UpdatedByMemberID: memberID(in.Membership), Now: s.Clock.Now(),
		})
		return err
	})
	if err != nil {
		return out, err
	}
	if s.Widgets == nil {
		return out, nil
	}
	return out, s.Widgets.ForgetAll(ctx, ws)
}

/*
apply checks the change and writes it over the current settings.
*/
func (s *Setup) apply(ctx context.Context, q *models.Queries, current models.IdentitySetting, in SetupChange,
) (models.IdentitySetting, error) {
	next := current
	if in.ClientWritable != nil {
		keys, err := writableKeys(*in.ClientWritable)
		if err != nil {
			return next, err
		}
		next.ClientWritableAttributes = keys
	}
	if in.Mode == nil || *in.Mode == current.Mode {
		return next, nil
	}
	if !slices.Contains([]string{ModeOff, ModeOptional, ModeRequired}, *in.Mode) {
		return next, fault.New(fault.Invalid, "mode_invalid", "The mode is off, optional or required.").
			OnField("mode")
	}
	next.Mode = *in.Mode
	switch {
	case next.Mode != ModeRequired:
		return next, nil
	case in.Force:
		return next, requireSecret(ctx, q, current.WorkspaceID)
	}
	return next, s.guard(ctx, q, current.WorkspaceID)
}

/*
requireSecret refuses required when no active secret can verify a token.
*/
func requireSecret(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	secrets, err := q.ListIdentitySecrets(ctx, ws)
	if err != nil {
		return fmt.Errorf("list identity secrets: %w", err)
	}
	if !slices.ContainsFunc(secrets, func(r models.IdentitySecret) bool { return r.Status == "active" }) {
		return ErrRequiredWithoutSecret
	}
	return nil
}

/*
guard refuses required without an active secret or without enough
signed-in starts.
*/
func (s *Setup) guard(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	if err := requireSecret(ctx, q, ws); err != nil {
		return err
	}
	summary, err := summarize(ctx, q, ws, sinceDay(s.Clock.Now(), GuardDays))
	if err != nil {
		return err
	}
	if summary.Starts() >= GuardStarts && summary.Share() >= GuardShare {
		return nil
	}
	return guardRefusal(summary)
}

/*
writableKeys checks the attribute keys a signed-in messenger may write:
at most 20 distinct snake_case keys.
*/
func writableKeys(keys []string) ([]string, error) {
	pattern := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !pattern.MatchString(k) {
			return nil, fault.New(fault.Invalid, "attribute_key_invalid",
				"Attribute keys are snake_case, such as plan_name.").OnField("client_writable_attributes")
		}
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	if len(out) > MaxClientWritable {
		return nil, fault.New(fault.Invalid, "too_many_attributes",
			"At most 20 attributes can be written by the messenger.").OnField("client_writable_attributes")
	}
	return out, nil
}
