/*
|--------------------------------------------------------------------------
| Conversation Scope
|--------------------------------------------------------------------------
|
| Which conversations a caller sees comes down to who holds a
| conversation and the caller's access. The same rule lives in the SQL
| function dimmi_conversation_visible, which queries call, and here, for
| a conversation already in hand; the two must always agree. A member's
| own assignment shows in every mode, so assigning someone grants them
| sight and reassigning takes it away.
|
| Actors with no person behind them, such as jobs and the agent, see
| everything: there is nobody to restrict.
|
*/

package authz

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationScope is what one caller sees of the workspace's
conversations. Mode is the access mode, MemberID the member whose own
assignments always show, nil for an actor with no person behind it,
TeamIDs the teams the mode names, and Unassigned whether a narrow mode
also shows conversations nobody holds. Trash says the caller may reach
deleted conversations at all.
*/
type ConversationScope struct {
	Mode       enums.ConversationAccess
	MemberID   *ids.UUID
	TeamIDs    []ids.UUID
	Unassigned bool
	Trash      bool
}

/*
VisibleParams are the scope's arguments to dimmi_conversation_visible,
which every query listing conversations takes under the names
access_mode, access_viewer, access_teams and access_unassigned. Teams is
never nil: a null array would make the function hide everything.
*/
type VisibleParams struct {
	Mode       string
	Viewer     *ids.UUID
	Teams      []ids.UUID
	Unassigned bool
}

/*
WorkspaceScope is the scope of an actor with no person behind it, who
sees every conversation of the workspace.
*/
func WorkspaceScope() ConversationScope {
	return ConversationScope{Mode: enums.ConversationAccessAll, Trash: true}
}

/*
ConversationScope returns what the caller sees: their member's access,
whichever credential they came through.
*/
func (m Membership) ConversationScope() ConversationScope {
	return MemberScope(m.Member, m.AccessTeams, m.Can("conversations:delete"))
}

/*
MemberScope returns what the member sees, given the teams their access
names and whether they may reach deleted conversations.
*/
func MemberScope(member models.WorkspaceMember, teamIDs []ids.UUID, trash bool) ConversationScope {
	return ConversationScope{
		Mode:       enums.ConversationAccess(member.ConversationAccess),
		MemberID:   &member.ID,
		TeamIDs:    teamIDs,
		Unassigned: member.ConversationAccessUnassigned,
		Trash:      trash,
	}
}

/*
LoadAccessTeams returns the teams the member's conversation access
names, reading them only for a mode that uses them, which most members'
does not.
*/
func LoadAccessTeams(ctx context.Context, q *models.Queries, member models.WorkspaceMember) ([]ids.UUID, error) {
	if !enums.ConversationAccess(member.ConversationAccess).UsesTeamList() {
		return nil, nil
	}

	teams, err := q.ListMemberAccessTeamIDs(ctx, models.ListMemberAccessTeamIDsParams{
		WorkspaceID: member.WorkspaceID, MemberID: member.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("load access teams: %w", err)
	}

	return teams, nil
}

/*
Sees reports whether a conversation held by the member and team, either
or both of which may be nobody, is in the scope.
*/
func (s ConversationScope) Sees(member, team *ids.UUID) bool {
	if member != nil && s.MemberID != nil && *member == *s.MemberID {
		return true
	}

	unheld := s.Unassigned && member == nil && team == nil

	switch s.Mode {
	case enums.ConversationAccessAll:
		return true
	case enums.ConversationAccessAssigned:
		return unheld
	case enums.ConversationAccessTeams:
		return unheld || (team != nil && slices.Contains(s.TeamIDs, *team))
	case enums.ConversationAccessAllExceptTeams:
		return team == nil || !slices.Contains(s.TeamIDs, *team)
	}

	return false
}

/*
Contains reports whether the conversation is in the scope: held in a way
the scope shows, and not deleted unless the caller may reach the trash.
*/
func (s ConversationScope) Contains(c models.Conversation) bool {
	if c.DeletedAt != nil && !s.Trash {
		return false
	}

	return s.Sees(c.AssigneeMemberID, c.AssigneeTeamID)
}

/*
Params returns the scope's arguments to dimmi_conversation_visible.
*/
func (s ConversationScope) Params() VisibleParams {
	teams := s.TeamIDs
	if teams == nil {
		teams = []ids.UUID{}
	}

	return VisibleParams{Mode: string(s.Mode), Viewer: s.MemberID, Teams: teams, Unassigned: s.Unassigned}
}
