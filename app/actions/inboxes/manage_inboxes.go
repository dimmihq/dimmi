/*
|--------------------------------------------------------------------------
| Manage Inboxes
|--------------------------------------------------------------------------
|
| Those who manage inboxes add them and change their name, default team
| and settings. A new inbox comes with
| its API channel, so conversations can be started in it through the
| REST API from the start.
|
*/

package inboxes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateInboxInput is who adds an inbox and its name.
*/
type CreateInboxInput struct {
	Membership authz.Membership
	Name       string
}

/*
CreateInbox adds an inbox with its API channel.

Rules:
  - The caller needs inboxes:manage.

Events: [appevents.InboxCreated].

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type CreateInbox struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the inbox and returns it.
*/
func (a *CreateInbox) Handle(ctx context.Context, in CreateInboxInput) (Inbox, error) {
	if !in.Membership.Can("inboxes:manage") {
		return Inbox{}, exceptions.ErrForbidden
	}

	var out Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now, ws := models.New(tx), a.Clock.Now(), in.Membership.Workspace.ID

		inbox, err := q.CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: ws, Name: in.Name, Settings: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create inbox: %w", err)
		}

		_, err = q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: ws, InboxID: inbox.ID, Type: "api", Name: "API", Status: "active",
			Config: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create api channel: %w", err)
		}

		out = Inbox{Inbox: inbox}

		return a.Events.Dispatch(ctx, tx, appevents.InboxCreated{
			WorkspaceID: ws, InboxID: inbox.ID, ActorID: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
UpdateInboxInput is who changes which inbox, and how: its new name, nil
to keep it; its default team, nil to keep it, a TeamChoice without an id
for none; and a merge patch on its settings, empty to keep them.
*/
type UpdateInboxInput struct {
	Membership  authz.Membership
	InboxID     ids.UUID
	Name        *string
	DefaultTeam *TeamChoice
	Settings    json.RawMessage
}

/*
TeamChoice is a chosen team, ID nil for none.
*/
type TeamChoice struct {
	ID *ids.UUID
}

/*
UpdateInbox renames an inbox and changes its default team and settings.

Rules:
  - The caller needs inboxes:manage.
  - Settings are patched, then checked as a whole: reopen window 0 to
    365 days, a #rrggbb color, a known assignment mode and queue order,
    capacities 1 to 500 on known channel types.
  - The default and overflow teams must exist and differ.
  - A change to nothing changes nothing.

Events: [appevents.InboxUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInboxSettingInvalid], [exceptions.ErrInboxTeamNotFound],
[exceptions.ErrOverflowIsDefaultTeam], or a wrapped database error.
*/
type UpdateInbox struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle changes the inbox and returns it as it now is.
*/
func (a *UpdateInbox) Handle(ctx context.Context, in UpdateInboxInput) (Inbox, error) {
	if !in.Membership.Can("inboxes:manage") {
		return Inbox{}, exceptions.ErrForbidden
	}

	var out Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inbox, err := find(ctx, q, in.Membership.Workspace.ID, in.InboxID)
		if err != nil {
			return err
		}

		saved, err := a.save(ctx, tx, inbox, in)
		if err != nil {
			return err
		}

		out, err = withDefault(ctx, q, saved)

		return err
	})

	return out, err
}

/*
save writes the planned change, when it changes anything, and returns
the inbox as it now is.
*/
func (a *UpdateInbox) save(
	ctx context.Context, tx *database.Tx, inbox models.Inbox, in UpdateInboxInput,
) (models.Inbox, error) {
	q := models.New(tx)

	next, err := planInbox(ctx, q, inbox, in)
	if err != nil || unchanged(inbox, next) {
		return inbox, err
	}

	saved, err := q.UpdateInboxDetails(ctx, models.UpdateInboxDetailsParams{
		Name: next.Name, DefaultTeamID: next.DefaultTeamID, Settings: next.Settings, Now: a.Clock.Now(),
		WorkspaceID: inbox.WorkspaceID, ID: inbox.ID,
	})
	if err != nil {
		return inbox, fmt.Errorf("update inbox: %w", err)
	}

	return saved, a.Events.Dispatch(ctx, tx, appevents.InboxUpdated{
		WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, ActorID: in.Membership.Member.ID,
		FromName: inbox.Name, ToName: saved.Name,
	})
}

/*
planInbox works out the inbox as the change leaves it, checked.
*/
func planInbox(ctx context.Context, q *models.Queries, inbox models.Inbox, in UpdateInboxInput) (models.Inbox, error) {
	next := inbox

	if in.Name != nil {
		next.Name = *in.Name
	}

	if in.DefaultTeam != nil {
		next.DefaultTeamID = in.DefaultTeam.ID
	}

	settings := inbox.ReadSettings()

	if len(in.Settings) > 0 {
		var err error
		if settings, next.Settings, err = mergeSettings(inbox.Settings, in.Settings); err != nil {
			return next, err
		}
	}

	if err := checkSettings(settings); err != nil {
		return next, err
	}

	return next, checkTeams(ctx, q, next, settings)
}

/*
unchanged reports whether the plan leaves the inbox as it is.
*/
func unchanged(a, b models.Inbox) bool {
	sameTeam := (a.DefaultTeamID == nil) == (b.DefaultTeamID == nil) &&
		(a.DefaultTeamID == nil || *a.DefaultTeamID == *b.DefaultTeamID)

	return a.Name == b.Name && sameTeam && bytes.Equal(a.Settings, b.Settings)
}
