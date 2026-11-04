/*
|--------------------------------------------------------------------------
| Delete Teams
|--------------------------------------------------------------------------
|
| Deleting a team happens in two steps. The team is hidden at once: it
| no longer lists, cannot take new conversations or members, and its
| name is free for a new team. In the same step its conversations, open,
| snoozed and closed, move where the admin chose: to another team, to
| a teammate, or to nobody, so none waits on a team that is gone. A
| sweep later purges the row, which takes the memberships with it.
| Purging only ever removes a team that was deleted first.
|
*/

package teams

import (
	"context"
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
DeleteTeamInput is who deletes a team, through their membership, the
team, and where its conversations go: ToTeam, keeping each one's
assignee, or ToMember, taking the team away; neither leaves them with
no team.
*/
type DeleteTeamInput struct {
	Membership authz.Membership
	TeamID     ids.UUID
	ToTeam     *ids.UUID
	ToMember   *ids.UUID
}

/*
DeleteTeam hides a live team of the caller's workspace.

Rules:
  - The caller needs members:write.
  - Deleted teams and teams of other workspaces are not found.
  - Every live conversation the team holds moves first: to ToTeam with
    its assignee kept, to ToMember with no team, or else off the team.
  - The target is another live team or an active member of the
    workspace, never both.
  - Inboxes that routed to the team route to ToTeam, or to none.

Events: [appevents.ConversationAssigned] for each conversation it held,
then [appevents.TeamDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrReassignToInvalid], or a wrapped database error.
*/
type DeleteTeam struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle hides the team.
*/
func (a *DeleteTeam) Handle(ctx context.Context, in DeleteTeamInput) error {
	if !in.Membership.Can(writePermission) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		team, err := lockTeam(ctx, q, in.Membership.Workspace.ID, in.TeamID)
		if err != nil {
			return err
		}

		if err := checkTarget(ctx, q, team, in); err != nil {
			return err
		}

		if err := a.move(ctx, tx, in, team); err != nil {
			return err
		}

		_, err = q.SoftDeleteTeam(ctx, models.SoftDeleteTeamParams{
			WorkspaceID: team.WorkspaceID, ID: team.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("delete team: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.TeamDeleted{
			WorkspaceID: team.WorkspaceID, TeamID: team.ID, DeletedBy: in.Membership.Member.ID,
		})
	})
}

/*
PurgeTeamInput names a deleted team by its workspace and id.
*/
type PurgeTeamInput struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
}

/*
PurgeTeam removes a deleted team for good, with its memberships, once
nothing is assigned to it any more. It runs inside the caller's
transaction when there is one, and the context must name the workspace
as its tenant.

Rules:
  - Only a team that was deleted first is removed; a live team, or one
    already gone, is left alone and reported as not purged.

Errors: a wrapped database error.
*/
type PurgeTeam struct {
	DB *database.DB
}

/*
Handle purges the team and reports whether it was removed.
*/
func (a *PurgeTeam) Handle(ctx context.Context, in PurgeTeamInput) (bool, error) {
	var purged int64

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		purged, err = models.New(tx).PurgeTeam(ctx, models.PurgeTeamParams{WorkspaceID: in.WorkspaceID, ID: in.TeamID})

		return err
	})
	if err != nil {
		return false, fmt.Errorf("purge team: %w", err)
	}

	return purged > 0, nil
}
