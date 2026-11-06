/*
|--------------------------------------------------------------------------
| AI Connect And Runs Requests
|--------------------------------------------------------------------------
|
| The connect callback's shape and the trace reads: thin carriers,
| the controller does the work.
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

/*
AIRunsRequest is the input of GET .../conversations/:id/ai/runs.
*/
type AIRunsRequest struct {
	Conversation string `path:"conversation"`
}

/*
AIRunRequest is the input of GET .../conversations/:id/ai/runs/:run.
*/
type AIRunRequest struct {
	Conversation string `path:"conversation"`
	Run          string `path:"run"`
}
