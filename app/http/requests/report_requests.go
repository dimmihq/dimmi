/*
|--------------------------------------------------------------------------
| Report Requests
|--------------------------------------------------------------------------
|
| Every report reads the same query string: from and to, whole days in
| the workspace's time zone with to included; compare, to set each
| number beside the period just before; business_hours, to count
| durations only while the inbox's office hours were open; and the
| filters, each a comma list: filter[inbox], filter[team],
| filter[member], filter[channel], filter[tag] and filter[priority],
| with filter[ai] narrowing to conversations the AI answered in or not.
|
*/

package requests

/*
ReportFilters are the range and filters every report reads.
*/
type ReportFilters struct {
	From          string `query:"from"             pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	To            string `query:"to"               pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
	Compare       bool   `query:"compare"`
	BusinessHours bool   `query:"business_hours"`
	Inbox         string `query:"filter[inbox]"    maxLength:"2000"`
	Team          string `query:"filter[team]"     maxLength:"2000"`
	Member        string `query:"filter[member]"   maxLength:"2000"`
	Channel       string `query:"filter[channel]"  maxLength:"500"`
	Tag           string `query:"filter[tag]"      maxLength:"2000"`
	Priority      string `query:"filter[priority]" maxLength:"100"`
	AI            string `query:"filter[ai]"       enum:"involved,not_involved,"`
}

/*
ShowReportRequest is the input of GET /reports/{report}.
*/
type ShowReportRequest struct {
	Report string `path:"report" maxLength:"64"`
	ReportFilters
}

/*
ReportConversationsRequest is the input of GET
/reports/{report}/conversations: the metric whose conversations to
list, the report's filters, and the page.
*/
type ReportConversationsRequest struct {
	Report string `path:"report"  maxLength:"64"`
	Metric string `query:"metric" maxLength:"64" required:"true"`
	Limit  int    `query:"limit"  minimum:"0" maximum:"100"`
	Offset int    `query:"offset" minimum:"0" maximum:"10000"`
	ReportFilters
}

/*
UpdateReportDigestRequest is the input of PATCH /report-digest: how
often the digest comes and what it covers. Fields left out keep their
value; a first digest needs its frequency.
*/
type UpdateReportDigestRequest struct {
	Body struct {
		Frequency     *string   `json:"frequency,omitempty"      enum:"daily,weekly"`
		Inboxes       *[]string `json:"inbox_ids,omitempty"      maxItems:"50"`
		Teams         *[]string `json:"team_ids,omitempty"       maxItems:"50"`
		Channels      *[]string `json:"channels,omitempty"       maxItems:"20"`
		BusinessHours *bool     `json:"business_hours,omitempty"`
	}
}
