/*
|--------------------------------------------------------------------------
| Workflow Requests
|--------------------------------------------------------------------------
|
| The inputs of workflows. A definition travels as plain JSON, exactly
| as an export holds it. A save names the lock version it started from,
| so two editors never overwrite each other.
|
*/

package requests

import "encoding/json"

/*
IndexWorkflowsRequest is the input of GET /workflows.
*/
type IndexWorkflowsRequest struct {
	Archived bool `query:"archived"`
}

/*
StoreWorkflowRequest is the input of POST /workflows: a name and a
trigger, or a whole definition to start from.
*/
type StoreWorkflowRequest struct {
	Body struct {
		Name        string          `json:"name"                  maxLength:"80"`
		Description *string         `json:"description,omitempty" maxLength:"500"`
		Trigger     string          `json:"trigger,omitempty"     maxLength:"40"`
		Spec        json.RawMessage `json:"spec,omitempty"`
	}
}

/*
WorkflowRequest names a workflow in the path.
*/
type WorkflowRequest struct {
	Workflow string `path:"workflow"`
}

/*
UpdateWorkflowRequest is the input of PATCH /workflows/{workflow}: a
save of the working copy, a status change, or a restore of a version.
*/
type UpdateWorkflowRequest struct {
	Workflow string `path:"workflow"`
	Body     struct {
		Name                    *string         `json:"name,omitempty"                       maxLength:"80"`
		Description             *string         `json:"description,omitempty"                maxLength:"500"`
		Spec                    json.RawMessage `json:"spec,omitempty"`
		LockVersion             int32           `json:"lock_version,omitempty"`
		TestOnTestConversations *bool           `json:"test_on_test_conversations,omitempty"`
		Status                  *string         `json:"status,omitempty"                     enum:"live,paused"`
		RestoreVersionID        *string         `json:"restore_version_id,omitempty"         maxLength:"64"`
		Publish                 bool            `json:"publish,omitempty"`
		AcknowledgeWarnings     bool            `json:"acknowledge_warnings,omitempty"`
	}
}

/*
WorkflowRunsRequest is the input of GET /workflows/{workflow}/runs: how
many of the newest runs to list.
*/
type WorkflowRunsRequest struct {
	Workflow string `path:"workflow"`
	Limit    int    `query:"limit" minimum:"0" maximum:"50"`
}
