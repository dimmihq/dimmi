/*
|--------------------------------------------------------------------------
| Conversation View Requests
|--------------------------------------------------------------------------
|
| Saved views are created whole and changed field by field; a field left
| out keeps its value, and an empty icon, sort or status clears it. The
| conditions are checked by the action, which knows every field and
| operator a view may use.
|
*/

package requests

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ConversationViewRequest names a saved view in the path.
*/
type ConversationViewRequest struct {
	View string `path:"view"`
}

/*
StoreConversationViewRequest is the input of POST /conversation-views.
*/
type StoreConversationViewRequest struct {
	Body struct {
		Name         string          `json:"name"                    validate:"required,max=60"`
		Icon         *string         `json:"icon,omitempty"          validate:"omitempty,max=40"`
		Scope        string          `json:"scope"                   validate:"required,oneof=shared personal"`
		Definition   json.RawMessage `json:"definition"`
		Sort         *string         `json:"sort,omitempty"          validate:"omitempty,max=60"`
		StatusFilter *string         `json:"status_filter,omitempty" validate:"omitempty,max=20"`
	}
}

/*
Validate refuses a name that is empty once cleaned, and a missing
definition.
*/
func (r *StoreConversationViewRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	if len(r.Body.Definition) == 0 || string(r.Body.Definition) == "null" {
		return violation(ctx, "definition", "required", "validation.required")
	}

	return nil
}

/*
UpdateConversationViewRequest is the input of PATCH
/conversation-views/{view}.
*/
type UpdateConversationViewRequest struct {
	View string `path:"view"`
	Body struct {
		Name         *string         `json:"name,omitempty"          validate:"omitempty,max=60"`
		Icon         *string         `json:"icon,omitempty"          validate:"omitempty,max=40"`
		Definition   json.RawMessage `json:"definition,omitempty"`
		Sort         *string         `json:"sort,omitempty"          validate:"omitempty,max=60"`
		StatusFilter *string         `json:"status_filter,omitempty" validate:"omitempty,max=20"`
	}
}

/*
Validate refuses a name that is empty once cleaned.
*/
func (r *UpdateConversationViewRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
ReorderConversationViewsRequest is the input of PATCH
/conversation-views: the shared views' ids in their new order.
*/
type ReorderConversationViewsRequest struct {
	Body struct {
		Order []string `json:"order" maxItems:"200" validate:"required,min=1"`
	}
}
