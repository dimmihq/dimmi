/*
|--------------------------------------------------------------------------
| Member Resources
|--------------------------------------------------------------------------
|
| A member as their teammates see them: their role and status in the
| workspace and who they are, by name and email, never anything about
| how they sign in, plus which conversations they see, which matters to
| whoever manages access. The caller's own membership, with the
| permissions they hold, is the separate member block of the me resource.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceMember is one member of a workspace, with their account. Teams
is present only when the request included the member's teams.
*/
type WorkspaceMember struct {
	ID           string  `json:"id"`
	Object       string  `json:"object"`
	Role         string  `json:"role"`
	Status       string  `json:"status"`
	Availability string  `json:"availability"`
	JoinedVia    *string `json:"joined_via"`

	ConversationAccess           string   `json:"conversation_access"`
	ConversationAccessUnassigned bool     `json:"conversation_access_unassigned"`
	AccessTeamIDs                []string `json:"access_team_ids"`

	User          Teammate       `json:"user"`
	Teams         *[]TeamSummary `json:"teams,omitempty"`
	DeactivatedAt *time.Time     `json:"deactivated_at"`
	CreatedAt     time.Time      `json:"created_at"`
}

/*
Teammate is the account behind a member, as other members see it.
*/
type Teammate struct {
	ID            string  `json:"id"`
	Object        string  `json:"object"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	AvatarURL     *string `json:"avatar_url"`
}

/*
NewWorkspaceMember shapes the member and their account.
*/
func NewWorkspaceMember(m models.WorkspaceMember, u models.User) WorkspaceMember {
	return WorkspaceMember{
		ID:           models.PublicID(models.PrefixMember, m.ID),
		Object:       "member",
		Role:         m.Role,
		Status:       m.Status,
		Availability: m.Availability,
		JoinedVia:    m.JoinedVia,

		ConversationAccess:           m.ConversationAccess,
		ConversationAccessUnassigned: m.ConversationAccessUnassigned,
		AccessTeamIDs:                []string{},

		User: Teammate{
			ID:            models.PublicID(models.PrefixUser, u.ID),
			Object:        "user",
			Name:          u.Name,
			Email:         u.Email,
			EmailVerified: u.IsVerified(),
		},
		DeactivatedAt: m.DeactivatedAt,
		CreatedAt:     m.CreatedAt,
	}
}

/*
PublicTeamIDs returns the public ids of the teams, an empty list for
none.
*/
func PublicTeamIDs(teamIDs []ids.UUID) []string {
	out := make([]string, len(teamIDs))
	for i, id := range teamIDs {
		out[i] = models.PublicID(models.PrefixTeam, id)
	}

	return out
}

/*
NewWorkspaceMembers shapes a list of members, in the order given.
*/
func NewWorkspaceMembers(rows []models.ListWorkspaceMembersRow) []WorkspaceMember {
	out := make([]WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewWorkspaceMember(row.WorkspaceMember, row.User))
	}

	return out
}
