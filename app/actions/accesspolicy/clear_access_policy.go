/*
|--------------------------------------------------------------------------
| Clear Access Policy
|--------------------------------------------------------------------------
|
| The way back in for a company whose own rules shut it out, such as an
| office address that changed under an allowlist. Only someone at the
| server runs it, so it asks nothing of the rules it clears, and it lands
| in the audit log as the system's doing.
|
*/

package accesspolicy

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OnlyIPAllowlist narrows clearing to the two address allowlists.
*/
const OnlyIPAllowlist = "ip_allowlist"

/*
ErrUnknownScope means the operator asked to clear something that is not
a part of the rules; ErrUnknownWorkspace that no workspace has the slug.
*/
var (
	ErrUnknownScope     = errors.New("clear access policy: unknown scope")
	ErrUnknownWorkspace = errors.New("clear access policy: no workspace with that slug")
)

/*
ClearAccessPolicyInput is the workspace to clear, by its slug, and how
much: OnlyIPAllowlist, or "" for every rule.
*/
type ClearAccessPolicyInput struct {
	Slug string
	Only string
}

/*
ClearAccessPolicy removes a workspace's rules on an operator's word.

Rules:
  - Requiring a second factor and single sign-on enforcement have their
    own switches and stay as they are.
  - Clearing nothing that is set changes nothing.

Events: [appevents.WorkspaceAccessPolicyChanged] when anything changed.

Errors: [ErrUnknownScope], [ErrUnknownWorkspace], or a wrapped database
error.
*/
type ClearAccessPolicy struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle clears the rules and returns the settings that changed.
*/
func (a *ClearAccessPolicy) Handle(ctx context.Context, in ClearAccessPolicyInput) (map[string]any, error) {
	keys, err := keysFor(in.Only)
	if err != nil {
		return nil, err
	}

	var changes map[string]any

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := q.FindWorkspaceBySlug(ctx, in.Slug)
		if errors.Is(err, database.ErrNotFound) {
			return ErrUnknownWorkspace
		}

		if err != nil {
			return fmt.Errorf("find workspace: %w", err)
		}

		w, err := q.LockWorkspace(ctx, found.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		changes = diff(w.Security(), cleared(w.Security(), keys), keys)
		if len(changes) == 0 {
			return nil
		}

		settings, err := models.WithAccessPolicy(w.Settings, keys)
		if err != nil {
			return err
		}

		_, err = q.UpdateWorkspace(ctx, models.UpdateWorkspaceParams{
			ID: w.ID, Name: w.Name, Slug: w.Slug, Timezone: w.Timezone, Locale: w.Locale, Settings: settings,
			Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("clear workspace rules: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.WorkspaceAccessPolicyChanged{
			WorkspaceID: w.ID, ChangedBy: ids.Nil(), Changes: changes,
		})
	})

	return changes, err
}

/*
keysFor returns the stored settings the scope empties.
*/
func keysFor(only string) (models.AccessPolicyChange, error) {
	lists := models.AccessPolicyChange{
		"dashboard_ip_allowlist": []models.AllowedAddress{}, "api_ip_allowlist": []models.AllowedAddress{},
	}

	switch only {
	case OnlyIPAllowlist:
		return lists, nil
	case "":
		lists["allowed_sign_in_methods"] = []string{}
		lists["google_allowed_domains"] = []string{}
		lists["session_max_age_minutes"] = nil
		lists["session_idle_timeout_minutes"] = nil

		return lists, nil
	default:
		return nil, ErrUnknownScope
	}
}

/*
cleared returns the rules with the keys emptied.
*/
func cleared(s models.SecuritySettings, keys models.AccessPolicyChange) models.SecuritySettings {
	s.DashboardAllowlist, s.APIAllowlist = nil, nil

	if _, all := keys["allowed_sign_in_methods"]; all {
		s.AllowedSignInMethods, s.GoogleAllowedDomains = nil, nil
		s.SessionMaxAge, s.SessionIdleTimeout = nil, nil
	}

	return s
}
