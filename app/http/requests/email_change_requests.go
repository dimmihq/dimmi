/*
|--------------------------------------------------------------------------
| Email Change Requests
|--------------------------------------------------------------------------
|
| Moving an account to a new address. The address and the token that
| confirms it both come in the body, never in the request's address, so
| neither reaches a log.
|
*/

package requests

/*
StoreEmailChangeRequest is the input of POST /email-changes.
*/
type StoreEmailChangeRequest struct {
	Body struct {
		Email string `json:"email,omitempty" validate:"required,email,max=254"`
	}
}

/*
UpdateEmailChangeRequest is the input of PATCH /email-changes/{change}:
the link's token.
*/
type UpdateEmailChangeRequest struct {
	Change string `path:"change"`
	Body   struct {
		Token string `json:"token,omitempty" validate:"required,max=256"`
	}
}
