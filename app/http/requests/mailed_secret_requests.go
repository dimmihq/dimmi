/*
|--------------------------------------------------------------------------
| Mailed Secret Requests
|--------------------------------------------------------------------------
|
| Asking for a link by mail, and redeeming one. A secret always arrives
| in the body, never in the address, so it is never written to a log
| on the way; a token in the query string is ignored. The public id of
| the row a link names comes in the path and proves nothing on its own.
|
*/

package requests

/*
StoreMagicLinkRequest is the input of POST /magic-links. Name names the
account when the address has none yet.
*/
type StoreMagicLinkRequest struct {
	Body struct {
		Email string `json:"email,omitempty" validate:"required,email,max=254"`
		Name  string `json:"name,omitempty" validate:"omitempty,max=80"`
	}
}

/*
StorePasswordResetRequest is the input of POST /password-resets.
*/
type StorePasswordResetRequest struct {
	Body struct {
		Email string `json:"email,omitempty" validate:"required,email,max=254"`
	}
}

/*
UpdatePasswordResetRequest is the input of PATCH
/password-resets/{reset}: the link's token and the new password.
*/
type UpdatePasswordResetRequest struct {
	Reset string `path:"reset"`
	Body  struct {
		Token    string `json:"token,omitempty"    validate:"required,max=256"`
		Password string `json:"password,omitempty" validate:"required,max=256,password_length,password"`
	}
}

/*
UpdateEmailVerificationRequest is the input of PATCH
/email-verifications/{verification}: the link's token.
*/
type UpdateEmailVerificationRequest struct {
	Verification string `path:"verification"`
	Body         struct {
		Token string `json:"token,omitempty" validate:"required,max=256"`
	}
}

/*
StoreUserConfirmationRequest is the input of POST /user/confirmation:
the method and the password or code it takes.
*/
type StoreUserConfirmationRequest struct {
	Body struct {
		Method   string `json:"method,omitempty"   validate:"required,max=40"`
		Password string `json:"password,omitempty" validate:"omitempty,max=256"`
		Code     string `json:"code,omitempty"     validate:"omitempty,max=20"`
	}
}
