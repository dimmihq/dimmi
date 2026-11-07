/*
|--------------------------------------------------------------------------
| Widget Session Refresh Requests
|--------------------------------------------------------------------------
|
| What a signed-in visitor's messenger sends to prove its customer again:
| a fresh identity token, and nothing else. The size limit sits above the
| token checks' own, so an oversized token gets its plain explanation.
|
*/

package requests

/*
UpdateWidgetSessionBody is the body of PATCH /widget/v1/session: a fresh
identity token for the session's customer.
*/
type UpdateWidgetSessionBody struct {
	_       struct{} `additionalProperties:"true"`
	UserJWT string   `json:"user_jwt" maxLength:"16384" minLength:"1"`
}

/*
UpdateWidgetSessionRequest is the input of PATCH /widget/v1/session.
*/
type UpdateWidgetSessionRequest struct {
	Body UpdateWidgetSessionBody
}
