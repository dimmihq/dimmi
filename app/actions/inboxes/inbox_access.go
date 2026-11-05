/*
|--------------------------------------------------------------------------
| Inbox Access
|--------------------------------------------------------------------------
|
| Who may open an inbox. An inbox open to everyone shows its conversations
| to every member, as far as their own access allows. A restricted inbox
| shows them only to the teams and teammates on its list, and to owners
| and admins, who open every inbox. Saving replaces the whole list, so
| what the page shows is exactly what holds.
|
*/

package inboxes

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
The ways an inbox can be open.
*/
const (
	AccessEveryone   = "everyone"
	AccessRestricted = "restricted"
)

/*
Access is who may open an inbox: everyone, or the teams and teammates on
its list.
*/
type Access struct {
	Mode    string
	Teams   []ids.UUID
	Members []ids.UUID
}

/*
SetInboxAccessInput is who changes which inbox's access, and to what.
*/
type SetInboxAccessInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
	Access     Access
}

/*
SetInboxAccess changes who may open an inbox.

Rules:
  - The caller needs inboxes:manage.
  - Access is everyone or restricted; an inbox open to everyone keeps no
    list.
  - Every team must exist, and every teammate be an active member.

Events: [appevents.InboxAccessChanged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInboxAccessInvalid], [exceptions.ErrInboxTeamNotFound],
[exceptions.ErrInboxAccessMemberNotFound], or a wrapped database error.
*/
type SetInboxAccess struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle saves the access and returns the inbox as it now is.
*/
func (a *SetInboxAccess) Handle(ctx context.Context, in SetInboxAccessInput) (Inbox, error) {
	if !in.Membership.Can("inboxes:manage") {
		return Inbox{}, exceptions.ErrForbidden
	}

	access, err := normalized(in.Access)
	if err != nil {
		return Inbox{}, err
	}

	var out Inbox

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inbox, err := find(ctx, q, in.Membership.Workspace.ID, in.InboxID)
		if err != nil {
			return err
		}

		if err := checkAccess(ctx, q, inbox.WorkspaceID, access); err != nil {
			return err
		}

		saved, err := a.save(ctx, q, inbox, access)
		if err != nil {
			return err
		}

		if out, err = withDefault(ctx, q, saved); err != nil {
			return err
		}

		out.Access = access

		return a.Events.Dispatch(ctx, tx, appevents.InboxAccessChanged{
			WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, ActorID: in.Membership.Member.ID, Name: inbox.Name,
			Access: access.Mode, Teams: access.Teams, Members: access.Members,
		})
	})

	return out, err
}

/*
save writes the mode and replaces the list.
*/
func (a *SetInboxAccess) save(
	ctx context.Context, q *models.Queries, inbox models.Inbox, access Access,
) (models.Inbox, error) {
	now := a.Clock.Now()

	saved, err := q.SetInboxAccessMode(ctx, models.SetInboxAccessModeParams{
		Access: access.Mode, Now: now, WorkspaceID: inbox.WorkspaceID, ID: inbox.ID,
	})
	if err != nil {
		return inbox, fmt.Errorf("set inbox access: %w", err)
	}

	err = q.ClearInboxAccess(ctx, models.ClearInboxAccessParams{WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID})
	if err != nil {
		return inbox, fmt.Errorf("clear inbox access: %w", err)
	}

	err = q.AddInboxAccessTeams(ctx, models.AddInboxAccessTeamsParams{
		WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, TeamIds: access.Teams, Now: now,
	})
	if err != nil {
		return inbox, fmt.Errorf("add inbox access teams: %w", err)
	}

	err = q.AddInboxAccessMembers(ctx, models.AddInboxAccessMembersParams{
		WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, MemberIds: access.Members, Now: now,
	})
	if err != nil {
		return inbox, fmt.Errorf("add inbox access members: %w", err)
	}

	return saved, nil
}

/*
normalized checks the mode, drops the list of an inbox open to everyone
and repeats in it, and never returns a nil list.
*/
func normalized(access Access) (Access, error) {
	if access.Mode != AccessEveryone && access.Mode != AccessRestricted {
		return access, exceptions.ErrInboxAccessInvalid
	}

	if access.Mode == AccessEveryone {
		return Access{Mode: AccessEveryone, Teams: []ids.UUID{}, Members: []ids.UUID{}}, nil
	}

	return Access{Mode: access.Mode, Teams: distinct(access.Teams), Members: distinct(access.Members)}, nil
}

/*
distinct returns the ids once each, in order.
*/
func distinct(list []ids.UUID) []ids.UUID {
	out := []ids.UUID{}
	for _, id := range list {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}

/*
checkAccess refuses a team or teammate the workspace does not have.
*/
func checkAccess(ctx context.Context, q *models.Queries, ws ids.UUID, access Access) error {
	for _, id := range access.Teams {
		_, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: ws, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrInboxTeamNotFound.OnField("teams")
		}

		if err != nil {
			return fmt.Errorf("find access team: %w", err)
		}
	}

	for _, id := range access.Members {
		member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: id})
		if errors.Is(err, database.ErrNotFound) || (err == nil && member.Status != "active") {
			return exceptions.ErrInboxAccessMemberNotFound
		}

		if err != nil {
			return fmt.Errorf("find access member: %w", err)
		}
	}

	return nil
}

/*
accessOf returns who may open each of the workspace's inboxes.
*/
func accessOf(
	ctx context.Context, q *models.Queries, ws ids.UUID, inboxes []models.Inbox,
) (map[ids.UUID]Access, error) {
	rows, err := q.ListInboxAccess(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list inbox access: %w", err)
	}

	out := make(map[ids.UUID]Access, len(inboxes))
	for _, inbox := range inboxes {
		out[inbox.ID] = Access{Mode: inbox.Access, Teams: []ids.UUID{}, Members: []ids.UUID{}}
	}

	for _, row := range rows {
		access, ok := out[row.InboxID]
		if !ok {
			continue
		}

		if row.TeamID != nil {
			access.Teams = append(access.Teams, *row.TeamID)
		}

		if row.MemberID != nil {
			access.Members = append(access.Members, *row.MemberID)
		}

		out[row.InboxID] = access
	}

	return out, nil
}
