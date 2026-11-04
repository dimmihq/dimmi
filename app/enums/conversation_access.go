/*
|--------------------------------------------------------------------------
| Conversation Access
|--------------------------------------------------------------------------
|
| How much of a workspace's conversations a member sees. Teams named by
| an access mode are an explicit list, never team membership, so joining
| a team never widens what someone sees without anyone deciding it.
|
*/

package enums

/*
ConversationAccess is a member's conversation access mode.
*/
type ConversationAccess string

/*
The access modes: every conversation, only those assigned to the
member, those plus the conversations of the listed teams, and everything
except the conversations of the listed teams.
*/
const (
	ConversationAccessAll            ConversationAccess = "all"
	ConversationAccessAssigned       ConversationAccess = "assigned"
	ConversationAccessTeams          ConversationAccess = "teams"
	ConversationAccessAllExceptTeams ConversationAccess = "all_except_teams"
)

/*
UsesTeamList reports whether the mode reads the member's list of teams.
*/
func (a ConversationAccess) UsesTeamList() bool {
	return a == ConversationAccessTeams || a == ConversationAccessAllExceptTeams
}
