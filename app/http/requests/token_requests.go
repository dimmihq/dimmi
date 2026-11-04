/*
|--------------------------------------------------------------------------
| Personal Access Token Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on personal access tokens. A token is named by
| its public id. Minting one takes a name, at least one ability and an
| optional expiry; renaming takes only the name. Whether the abilities
| fit the caller's role and the expiry fits the calendar is the action's
| to judge, since both depend on who asks and when.
|
*/

package requests

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
TokenRequest is the input of the endpoints on one token.
*/
type TokenRequest struct {
	Token string `path:"token"`
}

/*
StoreTokenRequest is the input of POST /tokens.
*/
type StoreTokenRequest struct {
	Body struct {
		Name      string     `json:"name"                 validate:"required,max=80"`
		Abilities []string   `json:"abilities"            validate:"required,min=1,max=40,dive,required,max=40"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
}

/*
Validate refuses a name made only of invisible characters, which would
be empty once cleaned.
*/
func (r *StoreTokenRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
UpdateTokenRequest is the input of PATCH /tokens/{token}.
*/
type UpdateTokenRequest struct {
	Token string `path:"token"`
	Body  struct {
		Name string `json:"name" validate:"required,max=80"`
	}
}

/*
Validate refuses a name that is empty once cleaned.
*/
func (r *UpdateTokenRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}
