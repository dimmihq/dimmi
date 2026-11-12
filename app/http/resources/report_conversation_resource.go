/*
|--------------------------------------------------------------------------
| Report Conversation Resources
|--------------------------------------------------------------------------
|
| The conversations behind a number, each with the value it added: the
| seconds of a duration, the score of a rating, the AI's cost in
| millionths of a dollar, or null for a count. And a teammate's report
| digest: how often it comes, what it covers and when it next goes.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReportConversation is one conversation behind a number.
*/
type ReportConversation struct {
	Object     string     `json:"object"`
	ID         string     `json:"id"`
	Number     int64      `json:"number"`
	Subject    *string    `json:"subject"`
	Status     string     `json:"status"`
	Channel    string     `json:"channel"`
	StartedAt  time.Time  `json:"started_at"`
	ClosedAt   *time.Time `json:"closed_at"`
	AssigneeID *string    `json:"assignee_id"`
	Value      *float64   `json:"value"`
}

/*
NewReportConversation shapes a drill-down row.
*/
func NewReportConversation(r reports.DrillRow) ReportConversation {
	var assignee *string
	if r.AssigneeMemberID != nil {
		assignee = new(models.PublicID(models.PrefixMember, *r.AssigneeMemberID))
	}

	return ReportConversation{
		Object: "report_conversation", ID: models.PublicID(models.PrefixConversation, r.ConversationID),
		Number: r.Number, Subject: r.Subject, Status: r.Status, Channel: r.Channel, StartedAt: r.StartedAt,
		ClosedAt: r.ClosedAt, AssigneeID: assignee, Value: r.Value,
	}
}

/*
ReportDigest is the caller's report digest; frequency is null when they
have none.
*/
type ReportDigest struct {
	Object        string     `json:"object"`
	Frequency     *string    `json:"frequency"     enum:"daily,weekly"`
	InboxIDs      []string   `json:"inbox_ids"`
	TeamIDs       []string   `json:"team_ids"`
	Channels      []string   `json:"channels"`
	BusinessHours bool       `json:"business_hours"`
	NextSendAt    *time.Time `json:"next_send_at"`
	LastSentAt    *time.Time `json:"last_sent_at"`
}

/*
NewReportDigest shapes a digest, an empty one when there is none (nil).
*/
func NewReportDigest(found *reports.Digest) ReportDigest {
	var d reports.Digest
	if found != nil {
		d = *found
	}

	out := ReportDigest{
		Object: "report_digest", InboxIDs: publicIDs(models.PrefixInbox, d.Filters.Inboxes),
		TeamIDs: publicIDs(models.PrefixTeam, d.Filters.Teams), Channels: d.Filters.Channels,
		BusinessHours: d.Filters.BusinessHours,
	}
	if out.Channels == nil {
		out.Channels = []string{}
	}

	if found != nil {
		out.Frequency, out.NextSendAt, out.LastSentAt = new(d.Frequency), new(d.NextSendAt), d.LastSentAt
	}

	return out
}
