/*
|--------------------------------------------------------------------------
| AI Connect Requests
|--------------------------------------------------------------------------
|
| The subscription connect's start and the callback's shape: thin
| carriers, the action does the work.
|
*/

package requests

/*
AIChatGPTStartRequest is the input of POST /ai/chatgpt/connect.
*/
type AIChatGPTStartRequest struct{}

/*
AIChatGPTCompleteRequest is the input of POST /ai/chatgpt/callback.
*/
type AIChatGPTCompleteRequest struct {
	Body struct {
		State string `json:"state" validate:"required"`
		Code  string `json:"code" validate:"required"`
	}
}
