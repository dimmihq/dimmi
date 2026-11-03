/*
|--------------------------------------------------------------------------
| Member Resources
|--------------------------------------------------------------------------
|
| A member as their teammates see them: their role and status in the
| workspace and who they are, by name and email, never anything about
| how they sign in. The caller's own membership, with the permissions
| they hold, is the separate member block of the me resource.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceMember is one member of a workspace, with their account.
*/
type WorkspaceMember struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	Availability  string     `json:"availability"`
	JoinedVia     *string    `json:"joined_via"`
	User          Teammate   `json:"user"`
	DeactivatedAt *time.Time `json:"deactivated_at"`
	CreatedAt     time.Time  `json:"created_at"`
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
NewWorkspaceMembers shapes a list of members, in the order given.
*/
func NewWorkspaceMembers(rows []models.ListWorkspaceMembersRow) []WorkspaceMember {
	out := make([]WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewWorkspaceMember(row.WorkspaceMember, row.User))
	}

	return out
}
