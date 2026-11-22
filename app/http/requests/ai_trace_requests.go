/*
|--------------------------------------------------------------------------
| AI Trace Requests
|--------------------------------------------------------------------------
|
| The reads behind the inbox's "why" panel and the settings' usage.
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
