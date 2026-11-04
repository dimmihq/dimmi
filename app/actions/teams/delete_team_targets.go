/*
|--------------------------------------------------------------------------
| Deleted Team Targets
|--------------------------------------------------------------------------
|
| Where a deleted team's conversations go. The admin picks another team,
| a teammate or nobody; each conversation then moves as the admin, on its
| own timeline, so the history says who moved it and why it changed
| hands. Closed conversations move too, so one that reopens has someone
| to land with.
|
*/

package teams

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
checkTarget refuses a target that is not one other live team or one
active member of the workspace.
*/
func checkTarget(ctx context.Context, q *models.Queries, team models.Team, in DeleteTeamInput) error {
	if in.ToTeam != nil && in.ToMember != nil {
		return exceptions.ErrReassignToInvalid
	}

	if in.ToTeam != nil {
		target, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: team.WorkspaceID, ID: *in.ToTeam})
		if errors.Is(err, database.ErrNotFound) || (err == nil && target.Team.ID == team.ID) {
			return exceptions.ErrReassignToInvalid
		}

		if err != nil {
			return fmt.Errorf("find team to move conversations to: %w", err)
		}
	}

	if in.ToMember == nil {
		return nil
	}

	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: team.WorkspaceID, ID: *in.ToMember,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && member.Status != string(enums.MemberStatusActive)) {
		return exceptions.ErrReassignToInvalid
	}

	if err != nil {
		return fmt.Errorf("find member to move conversations to: %w", err)
	}

	return nil
}

/*
move hands every live conversation of the team to the target as the
member deleting it, and points the inboxes that routed to the team at
the target team, or at none.
*/
func (a *DeleteTeam) move(ctx context.Context, tx *database.Tx, in DeleteTeamInput, team models.Team) error {
	w, err := conversations.NewWriter(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, in.Membership)
	if err != nil {
		return err
	}

	held, err := w.Q.ListTeamConversations(ctx, models.ListTeamConversationsParams{
		WorkspaceID: team.WorkspaceID, TeamID: team.ID,
	})
	if err != nil {
		return fmt.Errorf("list team conversations: %w", err)
	}

	for _, c := range held {
		if err := moveOne(ctx, w, c, in); err != nil {
			return err
		}
	}

	err = w.Q.MoveInboxesDefaultTeam(ctx, models.MoveInboxesDefaultTeamParams{
		ToTeamID: in.ToTeam, Now: a.Clock.Now(), WorkspaceID: team.WorkspaceID, TeamID: team.ID,
	})
	if err != nil {
		return fmt.Errorf("move inboxes off deleted team: %w", err)
	}

	return nil
}

/*
moveOne hands one conversation to the target.
*/
func moveOne(ctx context.Context, w *conversations.Writer, c models.Conversation, in DeleteTeamInput) error {
	switch {
	case in.ToTeam != nil:
		_, _, err := w.Assign(ctx, c, conversations.Assignment{Member: c.AssigneeMemberID, Team: in.ToTeam})

		return err
	case in.ToMember != nil:
		_, _, err := w.Assign(ctx, c, conversations.Assignment{Member: in.ToMember})

		return err
	default:
		_, err := w.DropTeam(ctx, c)

		return err
	}
}
