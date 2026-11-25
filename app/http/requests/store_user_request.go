/*
|--------------------------------------------------------------------------
| Store User Request
|--------------------------------------------------------------------------
|
| Signing up takes a name, an email, a password and optionally the name
| of a first workspace. A password must be long enough and must not be
| one of the passwords people use most, or the person's own email or
| name, which are the first things an attacker tries. A sign-up page
| that shows the legal terms says whether the person accepted them, and
| an install that publishes terms creates no account without that.
|
*/

package requests

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
StoreUserRequest is the input of POST /users.
*/
type StoreUserRequest struct {
	Body struct {
		Name            string `json:"name,omitempty"             validate:"required,max=80"`
		Email           string `json:"email,omitempty"            validate:"required,email,max=254"`
		Password        string `json:"password,omitempty"         validate:"required,max=256,password_length,password"`
		WorkspaceName   string `json:"workspace_name,omitempty"   validate:"omitempty,max=80"`
		InvitationToken string `json:"invitation_token,omitempty" validate:"omitempty,max=200"`
		CaptchaToken    string `json:"captcha_token,omitempty"    validate:"omitempty,max=4096"`
		TermsAccepted   bool   `json:"terms_accepted,omitempty"`
	}
}

/*
Validate refuses a name made only of invisible characters, which would
be empty once cleaned, an email address with characters no address can
have, and a password that is the person's own email or name.
*/
func (r *StoreUserRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	if _, err := mail.NormalizeAddress(r.Body.Email); err != nil {
		return violation(ctx, "email", "email", "validation.email")
	}

	if strings.EqualFold(r.Body.Password, r.Body.Email) || strings.EqualFold(r.Body.Password, r.Body.Name) {
		return violation(ctx, "password", "password", "validation.password_personal")
	}

	return nil
}

/*
TermsAccepted refuses a sign-up that did not accept the legal terms. An
install that publishes terms asks it; the field stays optional for one
that publishes none.
*/
func (r *StoreUserRequest) TermsAccepted(ctx context.Context) error {
	if r.Body.TermsAccepted {
		return nil
	}

	return violation(ctx, "terms_accepted", "accepted", "validation.terms_accepted")
}

/*
violation reports one broken rule on a field, with the message under
the key in the request's locale.
*/
func violation(ctx context.Context, field, rule, key string) error {
	return &validation.Error{Violations: []validation.Violation{{
		Field: field, Rule: rule, Message: i18n.T(ctx, key, i18n.Args{"field": field}),
	}}}
}
