/*
|--------------------------------------------------------------------------
| Audit Log Requests
|--------------------------------------------------------------------------
|
| The inputs of the audit log endpoints. Listing reads its filters,
| sort and cursor from the query string through the list parser, so only
| reading one entry has an input of its own.
|
*/

package requests

/*
ShowAuditLogRequest is the input of GET /audit-logs/{audit_log}.
*/
type ShowAuditLogRequest struct {
	AuditLog string `path:"audit_log"`
}
