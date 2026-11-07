/*
|--------------------------------------------------------------------------
| Widget Email Proof Requests
|--------------------------------------------------------------------------
|
| A messenger visitor asks for a code at an address, then sends the code
| back. The code arrives here from the card's form, never as a message.
|
*/

package requests

/*
SendWidgetEmailCodeRequest is the input of POST
/widget/v1/email-verifications.
*/
type SendWidgetEmailCodeRequest struct {
	Body struct {
		Email string `json:"email" maxLength:"254" minLength:"3"`
	}
}

/*
CheckWidgetEmailCodeRequest is the input of POST
/widget/v1/email-verifications/checks.
*/
type CheckWidgetEmailCodeRequest struct {
	Body struct {
		Code string `json:"code" maxLength:"12" minLength:"6"`
	}
}
