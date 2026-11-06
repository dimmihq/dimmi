/*
|--------------------------------------------------------------------------
| Saving Inbox Preferences
|--------------------------------------------------------------------------
|
| A save replaces the parts it sends and keeps the rest. Every view it
| names must be one a member can have, with a sort, status and layout a
| list accepts, so a typo never sits silently in someone's preferences.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
UpdatePreferencesInput is who saves which parts: each part that is not
nil replaces the saved one whole. MutationID is the id the client gave
the change.
*/
type UpdatePreferencesInput struct {
	Membership       authz.Membership
	Sidebar          json.RawMessage
	Views            map[string]ViewPreference
	PinnedAttributes []string
	MutationID       string
}

/*
UpdatePreferences saves the caller's inbox preferences.

Rules:
  - The caller needs conversations:read.
  - Every view key is a fixed view or the public id of a team, member,
    inbox or custom view; sorts and statuses are ones a list accepts.
  - The saved document is 64 KB at most.

Events: [appevents.InboxPreferencesUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidPreferences],
[exceptions.ErrPreferencesTooLarge], or a wrapped database error.
*/
type UpdatePreferences struct {
	DB     *database.DB
	IDs    *ids.Registry
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle saves the preferences and returns them as they now are.
*/
func (a *UpdatePreferences) Handle(ctx context.Context, in UpdatePreferencesInput) (Preferences, error) {
	if err := CanRead(in.Membership); err != nil {
		return Preferences{}, err
	}

	if err := validateViews(a.IDs, in.Views); err != nil {
		return Preferences{}, err
	}

	var out Preferences

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := preferenceStore{q: q, ids: a.IDs}.load(ctx, in.Membership)
		if err != nil {
			return err
		}

		out, err = a.save(ctx, q, in, merged(current, in))
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.InboxPreferencesUpdated{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: in.Membership.Workspace.ID, OccurredAt: a.Clock.Now(),
				Actor:            appevents.Actor{Type: "member", ID: in.Membership.Member.ID},
				ClientMutationID: in.MutationID,
			},
			MemberRef: appevents.MemberRef{MemberID: in.Membership.Member.ID},
		})
	})

	return out, err
}

/*
save writes the preferences, refusing a document over the size limit.
*/
func (a *UpdatePreferences) save(
	ctx context.Context, q *models.Queries, in UpdatePreferencesInput, p Preferences,
) (Preferences, error) {
	views, err := json.Marshal(p.Views)
	if err != nil {
		return p, fmt.Errorf("encode view preferences: %w", err)
	}

	pinned, err := json.Marshal(p.PinnedAttributes)
	if err != nil {
		return p, fmt.Errorf("encode pinned attributes: %w", err)
	}

	if len(p.Sidebar)+len(views)+len(pinned) > MaxPreferencesSize {
		return p, exceptions.ErrPreferencesTooLarge
	}

	row, err := q.SaveMemberInboxPreferences(ctx, models.SaveMemberInboxPreferencesParams{
		WorkspaceID: in.Membership.Workspace.ID, MemberID: in.Membership.Member.ID,
		Sidebar: p.Sidebar, Views: views, PinnedAttributes: pinned, Now: a.Clock.Now(),
	})
	if err != nil {
		return p, fmt.Errorf("save inbox preferences: %w", err)
	}

	p.UpdatedAt = &row.UpdatedAt

	return p, nil
}

/*
merged returns the saved preferences with the parts the input carries
in place of their saved ones.
*/
func merged(current Preferences, in UpdatePreferencesInput) Preferences {
	if in.Sidebar != nil {
		current.Sidebar = in.Sidebar
	}

	if in.Views != nil {
		current.Views = in.Views
	}

	if in.PinnedAttributes != nil {
		current.PinnedAttributes = in.PinnedAttributes
	}

	return current
}

/*
PeopleView is the preference key of the customer directory's people
list, whose saved columns are all it keeps.
*/
const PeopleView = "people"

/*
fixedViews returns the views every member has, and the people list of
the customer directory, which keeps its columns beside them.
*/
func fixedViews() []string {
	return []string{
		ViewMine, ViewUnassigned, ViewAll, ViewMentions, ViewSpam, ViewDeleted, string(enums.ConversationStatusSnoozed),
		PeopleView,
	}
}

/*
validateViews refuses a view key no view has, and a sort, status or
layout no list accepts.
*/
func validateViews(registry *ids.Registry, views map[string]ViewPreference) error {
	for key, view := range views {
		if !knownView(registry, key) {
			return exceptions.ErrInvalidPreferences.With("view", key)
		}

		if view.Sort != "" && !slices.Contains(querybuilder.Sorts(), querybuilder.Sort(view.Sort)) {
			return exceptions.ErrInvalidPreferences.With("view", key)
		}

		if view.Status != "" && !slices.Contains([]string{"open", "snoozed", "closed", "any"}, view.Status) {
			return exceptions.ErrInvalidPreferences.With("view", key)
		}

		if view.Layout != "" && view.Layout != "list" && view.Layout != "table" {
			return exceptions.ErrInvalidPreferences.With("view", key)
		}
	}

	return nil
}

/*
knownView reports whether the key is a fixed view or a well formed id
of a team, member, inbox or custom view.
*/
func knownView(registry *ids.Registry, key string) bool {
	if slices.Contains(fixedViews(), key) {
		return true
	}

	if _, ok := viewTarget(registry, key); ok {
		return true
	}

	_, err := registry.Decode(string(models.PrefixConversationView), key)

	return err == nil
}
