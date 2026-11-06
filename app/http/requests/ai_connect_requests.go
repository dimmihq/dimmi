/*
|--------------------------------------------------------------------------
| AI Connect Requests
|--------------------------------------------------------------------------
|
| The subscription connect's start and the callback's shape: thin
| carriers, the action does the work. A start names how the person
| signs in, by a code on another device or in a browser; a device
| sign-in polls the callback without a code until it is approved.
|
*/

package requests

/*
AIChatGPTStartRequest is the input of POST /ai/chatgpt/connect. Method
is "device", the default, or "browser".
*/
type AIChatGPTStartRequest struct {
	Body struct {
		Method string `json:"method,omitempty" validate:"omitempty,oneof=device browser"`
	} `required:"false"`
}

/*
AIChatGPTCompleteRequest is the input of POST /ai/chatgpt/callback.
Code is the browser sign-in's answer; a device sign-in leaves it out.
*/
type AIChatGPTCompleteRequest struct {
	Body struct {
		State string `json:"state" validate:"required"`
		Code  string `json:"code,omitempty" validate:"omitempty,max=4096"`
	}
}
