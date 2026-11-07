/*
|--------------------------------------------------------------------------
| Identity Token Check Requests
|--------------------------------------------------------------------------
|
| The token checker and signing a customer out everywhere. A pasted token
| is only ever checked: it is never stored, logged or replayed.
|
*/

package requests

/*
CheckIdentityTokenBody is the body of POST
/identity-verification/validations: the token and, optionally, the
messenger it is meant for.
*/
type CheckIdentityTokenBody struct {
	Token string `json:"token"            maxLength:"16384" minLength:"1"`
	AppID string `json:"app_id,omitempty" maxLength:"64"`
}

/*
CheckIdentityTokenRequest is the input of POST
/identity-verification/validations.
*/
type CheckIdentityTokenRequest struct {
	Body CheckIdentityTokenBody
}

/*
ContactWidgetSessionsRequest names the contact whose messenger sessions
end.
*/
type ContactWidgetSessionsRequest struct {
	Contact string `path:"contact"`
}
