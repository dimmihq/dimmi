/*
|--------------------------------------------------------------------------
| Workflow Analytics Requests
|--------------------------------------------------------------------------
|
| The inputs of a workflow's analytics, its failed runs and the workflows report.
|
*/

package requests

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

/*
WorkflowsReportRequest is the input of GET /reports/workflows: a range
of whole days, to included.
*/
type WorkflowsReportRequest struct {
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To   string `query:"to"   pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
}
