/*
|--------------------------------------------------------------------------
| Conversation Assignment
|--------------------------------------------------------------------------
|
| A conversation is held by at most one member and one team. Handing it
| over changes both in one step and says so once, from whom to whom, so
| a handover reads as one move in the timeline. Only an active member
| of the workspace can take a conversation, and only a team the
| workspace has.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Assignment is who should hold a conversation: the member and the team,
nil for nobody.
*/
type Assignment struct {
	Member *ids.UUID
	Team   *ids.UUID
}

/*
Assign hands the conversation to the assignment, and reports false when
it already held it.
*/
func (w *Writer) Assign(ctx context.Context, c models.Conversation, to Assignment) (models.Conversation, bool, error) {
	if sameID(c.AssigneeMemberID, to.Member) && sameID(c.AssigneeTeamID, to.Team) {
		return c, false, nil
	}

	members, err := w.people(ctx, c, to.Member)
	if err != nil {
		return c, false, err
	}

	teams, err := w.teams(ctx, c, to.Team)
	if err != nil {
		return c, false, err
	}

	updated, err := w.Q.SetConversationAssignment(ctx, models.SetConversationAssignmentParams{
		AssigneeMemberID: to.Member, AssigneeTeamID: to.Team, Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, false, fmt.Errorf("assign conversation: %w", err)
	}

	data := map[string]any{
		"from": map[string]any{"member": memberRef(c.AssigneeMemberID, members), "team": teamRef(teams.from)},
		"to":   map[string]any{"member": memberRef(to.Member, members), "team": teamRef(teams.to)},
	}

	return updated, true, w.emit(ctx, updated, entry{"conversation.assigned", data}, appevents.ConversationAssigned{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ActorID: w.Actor.ID,
		FromMemberID: orNil(c.AssigneeMemberID), ToMemberID: orNil(to.Member),
		FromTeamID: orNil(c.AssigneeTeamID), ToTeamID: orNil(to.Team),
	})
}

/*
people loads the member giving the conversation up and the one taking
it, refusing a taker who is not an active member.
*/
func (w *Writer) people(ctx context.Context, c models.Conversation, to *ids.UUID) (map[ids.UUID]Member, error) {
	var wanted []ids.UUID

	for _, id := range []*ids.UUID{c.AssigneeMemberID, to} {
		if id != nil {
			wanted = append(wanted, *id)
		}
	}

	members, err := Members(ctx, w.Q, c.WorkspaceID, wanted)
	if err != nil {
		return nil, fmt.Errorf("load assignees: %w", err)
	}

	if to == nil {
		return members, nil
	}

	if m, ok := members[*to]; !ok || m.Member.Status != string(enums.MemberStatusActive) {
		return nil, exceptions.ErrMemberInactive
	}

	return members, nil
}

/*
teamPair is the team giving a conversation up and the one taking it.
*/
type teamPair struct {
	from, to *models.Team
}

/*
teams loads the team giving the conversation up and the one taking it,
refusing a taker the workspace does not have.
*/
func (w *Writer) teams(ctx context.Context, c models.Conversation, to *ids.UUID) (teamPair, error) {
	var pair teamPair

	if c.AssigneeTeamID != nil {
		from, err := w.Q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: c.WorkspaceID, ID: *c.AssigneeTeamID})
		if err == nil {
			pair.from = &from.Team
		}
	}

	if to == nil {
		return pair, nil
	}

	team, err := w.Q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: c.WorkspaceID, ID: *to})
	if errors.Is(err, database.ErrNotFound) {
		return pair, exceptions.ErrTeamNotFound
	}

	if err != nil {
		return pair, fmt.Errorf("find team: %w", err)
	}

	pair.to = &team.Team

	return pair, nil
}

/*
sameID reports whether two optional ids name the same row, or both
nobody.
*/
func sameID(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
