/*
|--------------------------------------------------------------------------
| Auth Factor Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that manage the caller's own auth factors. A
| factor is named by its public id. Activating a factor with its first
| code is a step of its own, so a code never comes with other changes.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
StoreAuthFactorRequest is the input of POST /user/auth-factors: the
kind of factor to add, and its name, empty for the kind's default.
*/
type StoreAuthFactorRequest struct {
	Body struct {
		Type string `json:"type"           validate:"required,registered=second_factors"`
		Name string `json:"name,omitempty" validate:"omitempty,max=60"`
	}
}

/*
Validate refuses a name made only of invisible characters, which would
be empty once cleaned.
*/
func (r *StoreAuthFactorRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != "" && str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
UpdateAuthFactorRequest is the input of PATCH /user/auth-factors/{factor}:
the first code that activates a pending factor, or a new name, or
making it preferred.
*/
type UpdateAuthFactorRequest struct {
	Factor string `path:"factor"`
	Body   struct {
		Code      *string `json:"code,omitempty"      validate:"omitempty,max=20"`
		Name      *string `json:"name,omitempty"      validate:"omitempty,max=60"`
		Preferred *bool   `json:"preferred,omitempty"`
	}
}

/*
Validate refuses a name that is empty once cleaned.
*/
func (r *UpdateAuthFactorRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
Activates reports whether the request activates the factor with a code.
*/
func (r *UpdateAuthFactorRequest) Activates() bool {
	return r.Body.Code != nil
}

/*
DestroyAuthFactorRequest is the input of DELETE
/user/auth-factors/{factor}.
*/
type DestroyAuthFactorRequest struct {
	Factor string `path:"factor"`
}
