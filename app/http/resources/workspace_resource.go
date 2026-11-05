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
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Workspace is a workspace a member works in. RequireTwoFactor says
whether every member must have a second factor to work in it,
ShowViewers whether inbox lists show who views each conversation, and
SeenReceiptName whether the customer's widget names the teammate who
read their message. CustomerFacingName is the name customers see, null
when they see the workspace's own name. LinkSafety holds the checks the
inbox runs on links customers send, and Retention how long
conversations and their files are kept. DeletedAt and PurgeAfter are set
while it is in the trash: when it was deleted, and when it is purged for
good unless restored. UndoSendSeconds is how long replies wait before
they leave. Onboarding is how far its setup has come: the step reached,
and when it finished, null until then. LogoURL is the address of its
logo on this server, null without one.
*/
type Workspace struct {
	ID                 string            `json:"id"`
	Object             string            `json:"object"`
	Name               string            `json:"name"`
	Slug               string            `json:"slug"`
	LogoURL            *string           `json:"logo_url"`
	Timezone           string            `json:"timezone"`
	Locale             string            `json:"locale"`
	TestMode           bool              `json:"test_mode"`
	RequireTwoFactor   bool              `json:"require_two_factor"`
	ShowViewers        bool              `json:"show_viewers_on_list"`
	SeenReceiptName    bool              `json:"seen_receipt_shows_name"`
	CustomerFacingName *string           `json:"customer_facing_name"`
	LinkSafety         models.LinkSafety `json:"link_safety"`
	Retention          models.Retention  `json:"retention"`
	UndoSendSeconds    int               `json:"undo_send_seconds"`
	Onboarding         Onboarding        `json:"onboarding"`
	DeletedAt          *time.Time        `json:"deleted_at"`
	PurgeAfter         *time.Time        `json:"purge_after"`
}

/*
Onboarding is how far a workspace's setup has come.
*/
type Onboarding struct {
	Step        string     `json:"step"`
	CompletedAt *time.Time `json:"completed_at"`
}

/*
NewWorkspace shapes the workspace.
*/
func NewWorkspace(w models.Workspace) Workspace {
	return Workspace{
		ID:                 models.PublicID(models.PrefixWorkspace, w.ID),
		Object:             "workspace",
		Name:               w.Name,
		Slug:               w.Slug,
		LogoURL:            w.LogoURL(),
		Timezone:           w.Timezone,
		Locale:             w.Locale,
		TestMode:           w.TestMode,
		RequireTwoFactor:   w.RequiresTwoFactor(),
		ShowViewers:        w.ShowsViewersOnList(),
		SeenReceiptName:    w.SeenReceiptShowsName(),
		CustomerFacingName: w.CustomerFacingNameSetting(),
		LinkSafety:         w.LinkSafety(),
		Retention:          w.Retention(),
		UndoSendSeconds:    w.UndoSendSeconds(),
		Onboarding:         Onboarding{Step: w.OnboardingStep, CompletedAt: w.OnboardingCompletedAt},
		DeletedAt:          w.DeletedAt,
		PurgeAfter:         w.PurgeAfter,
	}
}

/*
WorkspaceSummary is a workspace in a list to switch between, with the
caller's role in it and the slugs it gave up and still holds, so a link
naming an old slug still finds it.
*/
type WorkspaceSummary struct {
	ID            string         `json:"id"`
	Object        string         `json:"object"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Role          string         `json:"role"`
	PreviousSlugs []PreviousSlug `json:"previous_slugs"`
}

/*
PreviousSlug is a slug a workspace gave up, held for it until a time.
*/
type PreviousSlug struct {
	Slug  string    `json:"slug"`
	Until time.Time `json:"until"`
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
			Role:   row.Role, PreviousSlugs: previousSlugs(row.PreviousSlugs),
		})
	}

	return summaries
}

/*
previousSlugs reads the slugs a workspace holds, none when they cannot
be read.
*/
func previousSlugs(raw json.RawMessage) []PreviousSlug {
	out := []PreviousSlug{}
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return []PreviousSlug{}
	}

	return out
}
