/*
|--------------------------------------------------------------------------
| Conversation Scope
|--------------------------------------------------------------------------
|
| Which conversations a caller sees comes down to the inbox a
| conversation sits in, who holds it, and the caller's access. A
| restricted inbox the caller may not open hides everything in it. The same rule lives in the SQL
| function dimmi_conversation_visible, which queries call, and here, for
| a conversation already in hand; the two must always agree. A member's
| own assignment shows in every mode and every inbox, so assigning
| someone grants them sight and reassigning takes it away.
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
also shows conversations nobody holds. HiddenInboxes are the restricted
inboxes the caller may not open. Trash says the caller may reach
deleted conversations at all.
*/
type ConversationScope struct {
	Mode          enums.ConversationAccess
	MemberID      *ids.UUID
	TeamIDs       []ids.UUID
	Unassigned    bool
	HiddenInboxes []ids.UUID
	Trash         bool
}

/*
VisibleParams are the scope's arguments to dimmi_conversation_visible,
which every query listing conversations takes under the names
access_mode, access_viewer, access_teams, access_unassigned and
access_hidden_inboxes. Teams and HiddenInboxes are never nil: a null
array would make the function hide everything.
*/
type VisibleParams struct {
	Mode          string
	Viewer        *ids.UUID
	Teams         []ids.UUID
	Unassigned    bool
	HiddenInboxes []ids.UUID
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
	s := MemberScope(m.Member, m.AccessTeams, m.Can("conversations:delete"))
	s.HiddenInboxes = m.HiddenInboxes

	return s
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
Restricted reports whether the scope hides any conversation.
*/
func (s ConversationScope) Restricted() bool {
	return s.Mode != enums.ConversationAccessAll || len(s.HiddenInboxes) > 0
}

/*
OpensInbox reports whether the inbox is one the scope may open.
*/
func (s ConversationScope) OpensInbox(inbox ids.UUID) bool {
	return !slices.Contains(s.HiddenInboxes, inbox)
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
Sees reports whether a conversation in the inbox, held by the member and
team, either or both of which may be nobody, is in the scope.
*/
func (s ConversationScope) Sees(inbox ids.UUID, member, team *ids.UUID) bool {
	if member != nil && s.MemberID != nil && *member == *s.MemberID {
		return true
	}

	if !s.OpensInbox(inbox) {
		return false
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

	return s.Sees(c.InboxID, c.AssigneeMemberID, c.AssigneeTeamID)
}

/*
Params returns the scope's arguments to dimmi_conversation_visible.
*/
func (s ConversationScope) Params() VisibleParams {
	return VisibleParams{
		Mode: string(s.Mode), Viewer: s.MemberID, Teams: orEmpty(s.TeamIDs), Unassigned: s.Unassigned,
		HiddenInboxes: orEmpty(s.HiddenInboxes),
	}
}

/*
orEmpty sends no list as an empty one, since the visibility rule hides
everything given a null array.
*/
func orEmpty(list []ids.UUID) []ids.UUID {
	if list == nil {
		return []ids.UUID{}
	}

	return list
}

/*
LoadHiddenInboxes returns the restricted inboxes the member may not
open. Whoever may manage inboxes opens every one.
*/
func LoadHiddenInboxes(ctx context.Context, q *models.Queries, member models.WorkspaceMember) ([]ids.UUID, error) {
	every, err := OpensEveryInbox(ctx, q, member)
	if err != nil || every {
		return nil, err
	}

	hidden, err := q.ListHiddenInboxIDs(ctx, models.ListHiddenInboxIDsParams{
		WorkspaceID: member.WorkspaceID, MemberID: member.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("load hidden inboxes: %w", err)
	}

	return hidden, nil
}

/*
OpensEveryInbox reports whether the member may manage inboxes, and so
opens every inbox whatever its list says.
*/
func OpensEveryInbox(ctx context.Context, q *models.Queries, member models.WorkspaceMember) (bool, error) {
	custom, err := q.CustomPermissions(ctx, member)
	if err != nil {
		return false, fmt.Errorf("load custom permissions: %w", err)
	}

	return slices.Contains(HeldPermissions(enums.Role(member.Role), custom), "inboxes:manage"), nil
}

/*
Reads reports whether the caller may read the conversation: it is in
their scope, or a note in it mentions them, since being asked in always
lets someone read along.
*/
func Reads(ctx context.Context, q *models.Queries, s ConversationScope, c models.Conversation) (bool, error) {
	if s.Contains(c) {
		return true, nil
	}

	if s.MemberID == nil || (c.DeletedAt != nil && !s.Trash) {
		return false, nil
	}

	mentioned, err := q.MentionsMember(ctx, models.MentionsMemberParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MemberID: *s.MemberID,
	})
	if err != nil {
		return false, fmt.Errorf("check mention: %w", err)
	}

	return mentioned, nil
}
