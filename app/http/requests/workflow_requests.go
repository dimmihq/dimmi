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

/*
WorkflowMetricsRequest is the input of GET
/workflows/{workflow}/metrics: a range of whole days, to included, the
last 30 when unset, and one version number or every version.
*/
type WorkflowMetricsRequest struct {
	Workflow string `path:"workflow"`
	From     string `query:"from"    pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To       string `query:"to"      pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	Version  int32  `query:"version" minimum:"0"`
}

/*
WorkflowFailuresRequest is the input of GET
/workflows/{workflow}/failures: a range of whole days, one step, and a
page of runs before a cursor.
*/
type WorkflowFailuresRequest struct {
	Workflow string `path:"workflow"`
	From     string `query:"from"    pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To       string `query:"to"      pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	StepID   string `query:"step_id" maxLength:"64"`
	Before   string `query:"before"  maxLength:"64"`
	Limit    int    `query:"limit"   minimum:"0"                      maximum:"50"`
}
