/*
|--------------------------------------------------------------------------
| Abuse Feedback Reports
|--------------------------------------------------------------------------
|
| Mailbox providers tell senders when a recipient marked their message
| as spam, in a standard report naming the kind of complaint and the
| original message. Senders must stop mailing a complaining recipient,
| so the report is read here to find who complained about what.
|
*/

package arf

import (
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/mailbox/internal/report"
)

/*
ErrNotARF rejects a message that is not an abuse feedback report.
*/
var ErrNotARF = report.ErrNotReport

/*
Feedback types a report can carry.
*/
const (
	TypeAbuse   = "abuse"
	TypeFraud   = "fraud"
	TypeVirus   = "virus"
	TypeOther   = "other"
	TypeNotSpam = "not-spam"
)

/*
Report is one complaint. FeedbackType is lowercased. OriginalRcptTo
lists the recipients who complained, when the provider names them.
OriginalMessageID is the Message-ID of the reported message, without
angle brackets, when the report includes its header.
*/
type Report struct {
	FeedbackType      string
	UserAgent         string
	Version           string
	OriginalMailFrom  string
	OriginalRcptTo    []string
	ArrivalDate       string
	SourceIP          string
	ReportedDomain    string
	OriginalMessageID string
}

/*
Complaint reports whether the recipient objected to the mail, as
opposed to reporting it as not spam.
*/
func (r *Report) Complaint() bool {
	return r.FeedbackType == TypeAbuse || r.FeedbackType == TypeFraud || r.FeedbackType == TypeVirus
}

/*
Parse reads an abuse feedback report.
*/
func Parse(raw []byte) (*Report, error) {
	read, err := report.Read(raw, "message/feedback-report")
	if err != nil {
		return nil, fmt.Errorf("parse arf: %w", err)
	}

	if len(read.Fields) == 0 || read.Fields[0].Get("Feedback-Type") == "" {
		return nil, fmt.Errorf("parse arf: %w: no feedback type", ErrNotARF)
	}

	fields := read.Fields[0]
	out := &Report{
		FeedbackType:      strings.ToLower(fields.Get("Feedback-Type")),
		UserAgent:         fields.Get("User-Agent"),
		Version:           fields.Get("Version"),
		OriginalMailFrom:  strings.Trim(fields.Get("Original-Mail-From"), "<>"),
		ArrivalDate:       fields.Get("Arrival-Date"),
		SourceIP:          fields.Get("Source-Ip"),
		ReportedDomain:    fields.Get("Reported-Domain"),
		OriginalMessageID: read.MessageID(),
	}

	for _, rcpt := range fields.Values("Original-Rcpt-To") {
		out.OriginalRcptTo = append(out.OriginalRcptTo, strings.Trim(rcpt, "<>"))
	}

	return out, nil
}
