/*
|--------------------------------------------------------------------------
| Store Session Request
|--------------------------------------------------------------------------
|
| Every way of signing in creates a session through the same endpoint;
| the grant says which way, and each grant has its own fields. An
| unknown grant is a field error like any other, so a client learns what
| went wrong without counting as a failed sign-in.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
The grants a session can be created with. Password is served today;
the others arrive with their flows.
*/
const (
	GrantPassword  = "password"
	GrantTwoFactor = "two_factor"
	GrantMagicLink = "magic_link"
	GrantEmailCode = "email_code"
)

/*
StoreSessionRequest is the input of POST /sessions. Reauth asks for a
fresh sign-in of the session's own user.
*/
type StoreSessionRequest struct {
	Body struct {
		Grant    string `json:"grant,omitempty"    validate:"required,oneof=password two_factor magic_link email_code"`
		Email    string `json:"email,omitempty"    validate:"omitempty,max=254"`
		Password string `json:"password,omitempty" validate:"omitempty,max=256"`
		Reauth   bool   `json:"reauth,omitempty"`
	}
}

/*
passwordGrant holds the fields the password grant requires.
*/
type passwordGrant struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

/*
Validate checks the fields of the chosen grant.
*/
func (r *StoreSessionRequest) Validate(ctx context.Context, v *validation.Factory) error {
	if r.Body.Grant != GrantPassword {
		return nil
	}

	return v.Struct(ctx, passwordGrant{Email: r.Body.Email, Password: r.Body.Password})
}
