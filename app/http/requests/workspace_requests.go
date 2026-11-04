/*
|--------------------------------------------------------------------------
| Workspace Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on the workspace a request works in and on the
| caller's own memberships. Every field of an update is optional and a
| missing one stays as it is. A name must keep something visible once
| cleaned, a slug must read well in an address, and a time zone and
| language must be ones the server can use.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
UpdateCurrentWorkspaceRequest is the input of PATCH /workspace.
*/
type UpdateCurrentWorkspaceRequest struct {
	Body struct {
		Name             *string `json:"name,omitempty"               validate:"omitempty,max=80"`
		Slug             *string `json:"slug,omitempty"               validate:"omitempty,min=3,max=40,slug"`
		Timezone         *string `json:"timezone,omitempty"           validate:"omitempty,timezone"`
		Locale           *string `json:"locale,omitempty"             validate:"omitempty,locale"`
		TestMode         *bool   `json:"test_mode,omitempty"`
		RequireTwoFactor *bool   `json:"require_two_factor,omitempty"`
		ShowViewers      *bool   `json:"show_viewers_on_list,omitempty"`
		SeenReceiptName  *bool   `json:"seen_receipt_shows_name,omitempty"`
	}
}

/*
Validate refuses a name made only of invisible characters, which would
be empty once cleaned.
*/
func (r *UpdateCurrentWorkspaceRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
DestroyMembershipRequest is the input of DELETE
/user/memberships/{membership}.
*/
type DestroyMembershipRequest struct {
	Membership string `path:"membership"`
}
