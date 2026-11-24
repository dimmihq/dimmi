/*
|--------------------------------------------------------------------------
| AI Agent Version Requests
|--------------------------------------------------------------------------
|
| Reading an agent's published versions and putting one back.
|
*/

package requests

/*
AIAgentVersionsRequest pages an agent's versions, newest first, below a
version number.
*/
type AIAgentVersionsRequest struct {
	Agent  string `path:"agent"`
	Before int32  `query:"before" minimum:"0"`
	Limit  int32  `query:"limit"  minimum:"0" maximum:"100"`
}

/*
AIAgentVersionRequest names one version of an agent by number.
*/
type AIAgentVersionRequest struct {
	Agent   string `path:"agent"`
	Version int32  `path:"version" minimum:"1"`
}

/*
AIRestoreAgentVersionBody says whether the restored version is
published at once.
*/
type AIRestoreAgentVersionBody struct {
	Publish bool `json:"publish,omitempty"`
}

/*
AIRestoreAgentVersionRequest is the input of POST
/ai/agents/{agent}/versions/{version}/restore.
*/
type AIRestoreAgentVersionRequest struct {
	Agent   string `path:"agent"`
	Version int32  `path:"version" minimum:"1"`
	Body    *AIRestoreAgentVersionBody
}
