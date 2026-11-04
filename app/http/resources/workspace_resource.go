/*
|--------------------------------------------------------------------------
| Workspace Resources
|--------------------------------------------------------------------------
|
| A workspace as its members see it, and the short form used in lists of
| workspaces to switch to. The organization a workspace belongs to is
| deliberately absent: it is an internal anchor, not something clients
| work with.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Workspace is a workspace a member works in. RequireTwoFactor says
whether every member must have a second factor to work in it. DeletedAt
and PurgeAfter are set while it is in the trash: when it was deleted,
and when it is purged for good unless restored.
*/
type Workspace struct {
	ID               string     `json:"id"`
	Object           string     `json:"object"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	Timezone         string     `json:"timezone"`
	Locale           string     `json:"locale"`
	TestMode         bool       `json:"test_mode"`
	RequireTwoFactor bool       `json:"require_two_factor"`
	DeletedAt        *time.Time `json:"deleted_at"`
	PurgeAfter       *time.Time `json:"purge_after"`
}

/*
NewWorkspace shapes the workspace.
*/
func NewWorkspace(w models.Workspace) Workspace {
	return Workspace{
		ID:               models.PublicID(models.PrefixWorkspace, w.ID),
		Object:           "workspace",
		Name:             w.Name,
		Slug:             w.Slug,
		Timezone:         w.Timezone,
		Locale:           w.Locale,
		TestMode:         w.TestMode,
		RequireTwoFactor: w.RequiresTwoFactor(),
		DeletedAt:        w.DeletedAt,
		PurgeAfter:       w.PurgeAfter,
	}
}

/*
WorkspaceSummary is a workspace in a list to switch between, with the
caller's role in it.
*/
type WorkspaceSummary struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Role   string `json:"role"`
}

/*
NewWorkspaceSummaries shapes the user's workspaces, in the order given.
*/
func NewWorkspaceSummaries(rows []models.ListUserWorkspacesRow) []WorkspaceSummary {
	summaries := make([]WorkspaceSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, WorkspaceSummary{
			ID:     models.PublicID(models.PrefixWorkspace, row.WorkspaceID),
			Object: "workspace",
			Name:   row.WorkspaceName,
			Slug:   row.WorkspaceSlug,
			Role:   row.Role,
		})
	}

	return summaries
}
