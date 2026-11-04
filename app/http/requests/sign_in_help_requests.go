/*
|--------------------------------------------------------------------------
| Sign-In Help Requests
|--------------------------------------------------------------------------
|
| Inputs of an admin helping a teammate back in, and of the teammate
| finishing it. The link's token and the password travel in the body,
| never in the request's address, so neither reaches a log.
|
*/

package requests

/*
MemberSignInHelpRequest is the input of POST
/members/{member}/two-factor-resets and DELETE
/members/{member}/sign-in-lock.
*/
type MemberSignInHelpRequest struct {
	Member string `path:"member"`
}

/*
UpdateTwoFactorResetRequest is the input of PATCH
/two-factor-resets/{reset}: the link's token, and the account's password
when it has one.
*/
type UpdateTwoFactorResetRequest struct {
	Reset string `path:"reset"`
	Body  struct {
		Token    string `json:"token,omitempty"    validate:"required,max=256"`
		Password string `json:"password,omitempty" validate:"omitempty,max=256"`
	}
}
