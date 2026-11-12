/*
|--------------------------------------------------------------------------
| Ticket Report Requests
|--------------------------------------------------------------------------
|
| Every ticket report reads a range of whole days, from and to, in the
| workspace's time zone with to included; without them the last thirty
| days. The field report also names a type and the key of its field.
|
*/

package requests

/*
TicketReportRequest is the input of the ticket reports.
*/
type TicketReportRequest struct {
	From string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"First day, 2006-01-02. Defaults to 30 days ago."`
	To   string `query:"to"   pattern:"^\\d{4}-\\d{2}-\\d{2}$" doc:"Last day included, 2006-01-02. Defaults to today."`
}

/*
TicketFieldReportRequest is the input of GET /reports/tickets/by-field.
*/
type TicketFieldReportRequest struct {
	TypeID string `query:"type_id" required:"true" maxLength:"64" doc:"The ticket type, tktt_..."`
	Field  string `query:"field"   required:"true" maxLength:"40" doc:"The key of one of the type's fields."`
	TicketReportRequest
}
