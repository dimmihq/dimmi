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

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
UpdateCurrentWorkspaceRequest is the input of PATCH /workspace. RawBody
tells a name for customers sent as null, which removes it, from one not
sent.
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
		UndoSendSeconds  *int    `json:"undo_send_seconds,omitempty"  validate:"omitempty,oneof=0 5 10 20 30"`
		SeenReceiptName  *bool   `json:"seen_receipt_shows_name,omitempty"`

		CustomerFacingName *string         `json:"customer_facing_name,omitempty" validate:"omitempty,max=80"`
		LinkSafety         *LinkSafetyBody `json:"link_safety,omitempty"`
		Retention          *RetentionBody  `json:"retention,omitempty"`
	}
	RawBody []byte
}

/*
LinkSafetyBody is a change to the workspace's link checks; a field left
out keeps its value, and a list sent replaces the list.
*/
type LinkSafetyBody struct {
	Enabled                *bool    `json:"enabled,omitempty"`
	AllowDomains           []string `json:"allow_domains,omitempty"             validate:"max=500,dive,max=253"`
	BlockDomains           []string `json:"block_domains,omitempty"             validate:"max=500,dive,max=253"`
	StripInboundEmailLinks *bool    `json:"strip_inbound_email_links,omitempty"`
}

/*
RetentionBody is a change to the retention periods, in days; a field
left out keeps its value, and closed_conversation_days set to null
keeps closed conversations for ever. The ranges are checked by the
action.
*/
type RetentionBody struct {
	ClosedConversationDays *int `json:"closed_conversation_days,omitempty"`
	SpamDays               *int `json:"spam_days,omitempty"`
	TrashDays              *int `json:"trash_days,omitempty"`
	RawEmailDays           *int `json:"raw_email_days,omitempty"`
	ExportDays             *int `json:"export_days,omitempty"`
}

/*
Validate refuses a name, or a name for customers, made only of invisible
characters, which would be empty once cleaned.
*/
func (r *UpdateCurrentWorkspaceRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	if r.Body.CustomerFacingName != nil && str.Clean(*r.Body.CustomerFacingName) == "" {
		return violation(ctx, "customer_facing_name", "required", "validation.required")
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

/*
UpdateWorkspaceLogoRequest is the input of PUT /workspace/logo: a
multipart form with the picture in the logo field.
*/
type UpdateWorkspaceLogoRequest struct {
	RawBody fhttp.MultipartForm
}
