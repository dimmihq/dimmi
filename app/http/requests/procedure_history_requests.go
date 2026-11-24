/*
|--------------------------------------------------------------------------
| Procedure History Requests
|--------------------------------------------------------------------------
|
| The published versions of a procedure, one of them, restoring one, the
| scorecard's range and the procedures report's days.
|
*/

package requests

/*
ProcedureVersionsRequest is the input of GET
/procedures/{procedure}/versions: a page of versions below a number.
*/
type ProcedureVersionsRequest struct {
	Procedure string `path:"procedure"`
	Before    int32  `query:"before" minimum:"0"`
	Limit     int32  `query:"limit"  minimum:"0" maximum:"100"`
}

/*
ProcedureVersionRequest names one version of a procedure by number.
*/
type ProcedureVersionRequest struct {
	Procedure string `path:"procedure"`
	Version   int32  `path:"version" minimum:"1"`
}

/*
RestoreProcedureVersionRequest is the input of POST
/procedures/{procedure}/versions/{version}/restore: publish at once, or
only put it back as the working copy, from the lock version the editor
started from.
*/
type RestoreProcedureVersionRequest struct {
	Procedure string `path:"procedure"`
	Version   int32  `path:"version" minimum:"1"`
	Body      *struct {
		Publish     bool  `json:"publish,omitempty"`
		LockVersion int32 `json:"lock_version,omitempty"`
	}
}

/*
ProcedureMetricsRequest is the input of GET
/procedures/{procedure}/metrics: the last 7, 30 or 90 days, for every
version or one.
*/
type ProcedureMetricsRequest struct {
	Procedure string `path:"procedure"`
	Days      int    `query:"days"    minimum:"0" maximum:"90"`
	Version   string `query:"version" maxLength:"10"`
}

/*
ProceduresReportRequest is the input of GET /reports/procedures: a
range of whole days, to included.
*/
type ProceduresReportRequest struct {
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To   string `query:"to"   pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
}

/*
WorkflowsReportRequest is the input of GET /reports/workflows: a range
of whole days, to included.
*/
type WorkflowsReportRequest struct {
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To   string `query:"to"   pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
}
