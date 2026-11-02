/*
|--------------------------------------------------------------------------
| Delivery Status Notifications
|--------------------------------------------------------------------------
|
| When a message cannot be delivered, the server that gave up sends a
| report back: per recipient, what it did and the status code it got.
| Reading the report tells a hard bounce, never to be retried, from a
| delay that may still resolve, and names the original message so the
| bounce can be matched to what was sent.
|
| Bounces that do not follow the standard are not guessed at here: they
| are refused, and the caller decides whether to look further.
|
*/

package dsn

import (
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/mailbox/internal/report"
)

/*
ErrNotDSN rejects a message that is not a standard delivery status
notification.
*/
var ErrNotDSN = report.ErrNotReport

/*
Recipient is the outcome for one recipient.

Action is failed, delayed, delivered, relayed or expanded, lowercased.
Status is the enhanced status code such as 5.1.1. Addresses and the
diagnostic code are given without their type label, as in
"550 5.1.1 User unknown".
*/
type Recipient struct {
	FinalRecipient    string
	OriginalRecipient string
	Action            string
	Status            string
	DiagnosticCode    string
	RemoteMTA         string
}

/*
Permanent reports a hard bounce: delivery failed with a 5.x.x status.
*/
func (r Recipient) Permanent() bool {
	return r.Action == "failed" && strings.HasPrefix(r.Status, "5")
}

/*
Report is one delivery status notification. OriginalMessageID is the
Message-ID of the bounced message, without angle brackets, when the
report includes its header.
*/
type Report struct {
	ReportingMTA       string
	OriginalEnvelopeID string
	OriginalMessageID  string
	Recipients         []Recipient
}

/*
Parse reads a delivery status notification.
*/
func Parse(raw []byte) (*Report, error) {
	read, err := report.Read(raw, "message/delivery-status", "message/global-delivery-status")
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	out := &Report{OriginalMessageID: read.MessageID()}

	for _, block := range read.Fields {
		if block.Get("Final-Recipient") == "" {
			out.ReportingMTA = report.Value(block.Get("Reporting-MTA"))
			out.OriginalEnvelopeID = block.Get("Original-Envelope-Id")

			continue
		}

		out.Recipients = append(out.Recipients, Recipient{
			FinalRecipient:    report.Value(block.Get("Final-Recipient")),
			OriginalRecipient: report.Value(block.Get("Original-Recipient")),
			Action:            strings.ToLower(block.Get("Action")),
			Status:            block.Get("Status"),
			DiagnosticCode:    report.Value(block.Get("Diagnostic-Code")),
			RemoteMTA:         report.Value(block.Get("Remote-MTA")),
		})
	}

	if len(out.Recipients) == 0 {
		return nil, fmt.Errorf("parse dsn: %w: no recipients", ErrNotDSN)
	}

	return out, nil
}
