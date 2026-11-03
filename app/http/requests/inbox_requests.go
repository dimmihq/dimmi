/*
|--------------------------------------------------------------------------
| Inbox Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on inboxes. Every field of an update is
| optional and a missing one stays as it is.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ShowInboxRequest is the input of GET /inboxes/{inbox}.
*/
type ShowInboxRequest struct {
	Inbox string `path:"inbox"`
}

/*
StoreInboxRequest is the input of POST /inboxes.
*/
type StoreInboxRequest struct {
	Body struct {
		Name string `json:"name" validate:"required,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *StoreInboxRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
UpdateInboxRequest is the input of PATCH /inboxes/{inbox}.
*/
type UpdateInboxRequest struct {
	Inbox string `path:"inbox"`
	Body  struct {
		Name *string `json:"name,omitempty" validate:"omitempty,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *UpdateInboxRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}
