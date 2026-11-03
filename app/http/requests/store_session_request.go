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
The grants a session can be created with: a password, a mailed link's
token, a mailed code with its email, and the second factor that
completes a sign-in.
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
		Token    string `json:"token,omitempty"    validate:"omitempty,max=256"`
		Code     string `json:"code,omitempty"     validate:"omitempty,max=20"`
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
magicLinkGrant holds the field the magic link grant requires.
*/
type magicLinkGrant struct {
	Token string `json:"token" validate:"required"`
}

/*
emailCodeGrant holds the fields the email code grant requires.
*/
type emailCodeGrant struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code"  validate:"required"`
}

/*
Validate checks the fields of the chosen grant.
*/
func (r *StoreSessionRequest) Validate(ctx context.Context, v *validation.Factory) error {
	switch r.Body.Grant {
	case GrantPassword:
		return v.Struct(ctx, passwordGrant{Email: r.Body.Email, Password: r.Body.Password})
	case GrantMagicLink:
		return v.Struct(ctx, magicLinkGrant{Token: r.Body.Token})
	case GrantEmailCode:
		return v.Struct(ctx, emailCodeGrant{Email: r.Body.Email, Code: r.Body.Code})
	default:
		return nil
	}
}
