/*
|--------------------------------------------------------------------------
| AI Trace Requests
|--------------------------------------------------------------------------
|
| The reads behind the inbox's "why" panel and the settings' usage, and
| the playground's test question.
|
*/

package requests

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

/*
AIUsageRequest names the month, as YYYY-MM; empty is the current one.
*/
type AIUsageRequest struct {
	Period string `query:"period"`
}

/*
AIPlaygroundRequest is the input of POST /ai/playground: a test
question and, optionally, the conversation whose history it reads.
*/
type AIPlaygroundRequest struct {
	Body struct {
		Message      string `json:"message"                validate:"required,max=4000"`
		Conversation string `json:"conversation,omitempty" validate:"omitempty,max=64"`
	}
}
