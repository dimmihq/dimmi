/*
|--------------------------------------------------------------------------
| Workspace Lifecycle Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that delete and restore a workspace. Deleting
| asks for the workspace's address as confirmation. Restoring takes
| exactly one change, trashed: false; every other workspace field
| changes through PATCH /workspace, so it is refused here rather than
| ignored.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
DestroyCurrentWorkspaceRequest is the input of DELETE /workspace.
*/
type DestroyCurrentWorkspaceRequest struct {
	Body struct {
		ConfirmSlug string `json:"confirm_slug,omitempty" validate:"required,max=80"`
	}
}

/*
UpdateWorkspaceRequest is the input of PATCH /workspaces/{workspace}.
The fields besides Trashed are only there to be refused.
*/
type UpdateWorkspaceRequest struct {
	Workspace string `path:"workspace"`
	Body      struct {
		Trashed          *bool `json:"trashed,omitempty"`
		Name             any   `json:"name,omitempty"`
		Slug             any   `json:"slug,omitempty"`
		Timezone         any   `json:"timezone,omitempty"`
		Locale           any   `json:"locale,omitempty"`
		TestMode         any   `json:"test_mode,omitempty"`
		RequireTwoFactor any   `json:"require_two_factor,omitempty"`
		ShowViewers      any   `json:"show_viewers_on_list,omitempty"`
	}
}

/*
Validate requires trashed: false and refuses every other field.
*/
func (r *UpdateWorkspaceRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body

	others := []struct {
		name  string
		value any
	}{
		{"name", b.Name},
		{"slug", b.Slug},
		{"timezone", b.Timezone},
		{"locale", b.Locale},
		{"test_mode", b.TestMode},
		{"require_two_factor", b.RequireTwoFactor},
		{"show_viewers_on_list", b.ShowViewers},
	}
	for _, field := range others {
		if field.value != nil {
			return violation(ctx, field.name, "prohibited", "validation.prohibited")
		}
	}

	if b.Trashed == nil {
		return violation(ctx, "trashed", "required", "validation.required")
	}

	if *b.Trashed {
		return violation(ctx, "trashed", "prohibited", "validation.prohibited")
	}

	return nil
}
