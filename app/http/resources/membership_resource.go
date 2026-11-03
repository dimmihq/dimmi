/*
|--------------------------------------------------------------------------
| Membership Resource
|--------------------------------------------------------------------------
|
| The caller's own place in a workspace, with the workspace itself, so
| an app can list where its user belongs and open the one used last
| without another request. A workspace in the trash shows when it will
| be purged.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Membership is one of the caller's memberships. LastAccessedAt is null
for a workspace never opened.
*/
type Membership struct {
	ID             string              `json:"id"`
	Object         string              `json:"object"`
	Role           string              `json:"role"`
	Status         string              `json:"status"`
	JoinedVia      *string             `json:"joined_via"`
	LastAccessedAt *time.Time          `json:"last_accessed_at"`
	Workspace      MembershipWorkspace `json:"workspace"`
}

/*
MembershipWorkspace is the workspace of a membership, with its trash
state.
*/
type MembershipWorkspace struct {
	ID         string     `json:"id"`
	Object     string     `json:"object"`
	Name       string     `json:"name"`
	Slug       string     `json:"slug"`
	DeletedAt  *time.Time `json:"deleted_at"`
	PurgeAfter *time.Time `json:"purge_after"`
}

/*
NewMemberships shapes the caller's memberships, in the order given.
*/
func NewMemberships(rows []models.ListUserMembershipsRow) []Membership {
	out := make([]Membership, 0, len(rows))

	for _, row := range rows {
		m := Membership{
			ID:     models.PublicID(models.PrefixMember, row.MemberID),
			Object: "membership",
			Role:   row.Role,
			Status: row.Status,
			Workspace: MembershipWorkspace{
				ID:         models.PublicID(models.PrefixWorkspace, row.Workspace.ID),
				Object:     "workspace",
				Name:       row.Workspace.Name,
				Slug:       row.Workspace.Slug,
				DeletedAt:  row.Workspace.DeletedAt,
				PurgeAfter: row.Workspace.PurgeAfter,
			},
		}

		if row.JoinedVia != "" {
			m.JoinedVia = &row.JoinedVia
		}

		if row.Accessed {
			m.LastAccessedAt = &row.LastAccessedAt
		}

		out = append(out, m)
	}

	return out
}
