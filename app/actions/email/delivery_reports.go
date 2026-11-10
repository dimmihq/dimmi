/*
|--------------------------------------------------------------------------
| Delivery Reports
|--------------------------------------------------------------------------
|
| Bounces and spam complaints arrive as mail about mail the workspace
| sent. Each is matched to the sent email by its Message-ID, and only
| ever acts on addresses that email went to, so a forged report cannot
| silence anyone else. A bounce saying the address does not exist stops
| mail to it until its owner writes in again; any other refusal, such as
| a full mailbox or a policy block, stops it for a few days only. Both
| fail the reply meant for the address; a delay is only noted. A
| complaint stops all mail to the complainer, the recipients it names
| when it names any. A report about nothing the workspace sent is
| recorded as unmatched.
|
| A mail system's notice without a readable report is still matched,
| through the ids it quotes, and fails the reply it names. It suppresses
| nothing, because nothing in it says the address is gone for good.
|
| A bounced forwarding test fails that test and suppresses nothing: the
| address it bounced from is the workspace's own.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mailbox/dsn"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
quotedMessageID finds a Message-ID line a notice quotes from the mail
that failed.
*/
var quotedMessageID = regexp.MustCompile(`(?im)^[ \t>]*message-id:\s*<([^<>\s]+)>`)

/*
lapsingBounce is how long a refusal that does not say the address is
gone stops mail to it.
*/
const lapsingBounce = 72 * time.Hour

/*
addressGone reports whether a delivery status code says the address
itself does not exist or cannot be reached at all.
*/
func addressGone(status string) bool {
	switch status {
	case "5.1.0", "5.1.1", "5.1.2", "5.1.3", "5.1.6", "5.1.10":
		return true
	}

	return false
}

/*
noticeDiagnostic explains a failure read from a notice with no report.
*/
const noticeDiagnostic = "the receiving mail system said the email could not be delivered"

/*
outcome is one recipient's fate to record: the address, what became of
it, the suppression it causes, if any, when that lapses, and the status
of the whole email afterwards.
*/
type outcome struct {
	address string
	status  recipientStatus
	reason  enums.EmailSuppressionReason
	source  string
	expires *time.Time
	overall string
}

/*
reportRun records a report's outcomes on one sent email.
*/
type reportRun struct {
	a    *ProcessInboundEmail
	tx   *database.Tx
	q    *models.Queries
	sent models.OutboundEmail
}

/*
complaint stops mail to whoever reported the workspace's email as spam.
*/
func (a *ProcessInboundEmail) complaint(ctx context.Context, mail *received) error {
	report := mail.feedback
	if !report.Complaint() {
		return a.ignore(ctx, mail.row, ignoreComplaintNotSpam)
	}

	reason := ignoreComplaint + unmatchedSuffix

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		sent, ok, err := findSent(ctx, q, mail.row.WorkspaceID, report.OriginalMessageID)
		if err != nil || !ok {
			return err
		}

		reason = ignoreComplaint
		run := reportRun{a: a, q: q, sent: sent}
		complained := string(enums.OutboundEmailStatusComplained)

		for _, recipient := range complainers(report.OriginalRcptTo, sent) {
			err := run.record(ctx, outcome{
				address: recipient, status: recipientStatus{Status: complained, At: a.Clock.Now()},
				reason: enums.EmailSuppressionReasonComplaint, source: "arf", overall: complained,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return a.ignore(ctx, mail.row, reason)
}

/*
bounce records a delivery report on the email it is about.
*/
func (a *ProcessInboundEmail) bounce(ctx context.Context, mail *received) error {
	report := mail.delivery
	if report == nil {
		return a.noticeBounce(ctx, mail)
	}

	original := report.OriginalMessageID
	if original == "" && len(mail.parsed.InReplyTo) > 0 {
		original = mail.parsed.InReplyTo[0]
	}

	reason := ignoreBounceUnmatched

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		sent, ok, err := findSent(ctx, q, mail.row.WorkspaceID, original)
		if err != nil || !ok {
			return err
		}

		reason = ignoreBounce
		run := reportRun{a: a, tx: tx, q: q, sent: sent}

		for _, r := range report.Recipients {
			if err := run.bounced(ctx, r); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return a.ignore(ctx, mail.row, reason)
}

/*
bounced records one recipient of a delivery report, when the sent email
went to it. A permanent failure stops mail to the address, for good
when the address is gone and for a few days otherwise, and fails the
reply when it was meant for it.
*/
func (run reportRun) bounced(ctx context.Context, r dsn.Recipient) error {
	address := lower(r.FinalRecipient)
	if !sentTo(run.sent, address) {
		return nil
	}

	status := recipientStatus{
		Status: "delayed", Code: r.Status, Diagnostic: diagnostic(r.DiagnosticCode), At: run.a.Clock.Now(),
	}

	if !r.Permanent() {
		return run.record(ctx, outcome{address: address, status: status, overall: run.sent.Status})
	}

	status.Status = string(enums.OutboundEmailStatusBounced)

	if run.sent.Kind == string(enums.OutboundEmailKindTest) {
		return run.testBounced(ctx, address, status)
	}

	failed := outcome{
		address: address, status: status, reason: enums.EmailSuppressionReasonHardBounce, source: "dsn",
		overall: status.Status,
	}

	if !addressGone(r.Status) {
		failed.reason = enums.EmailSuppressionReasonSoftBounce
		failed.expires = new(run.a.Clock.Now().Add(lapsingBounce))
	}

	err := run.record(ctx, failed)
	if err != nil {
		return err
	}

	return run.failReply(ctx, address, status.Diagnostic)
}

/*
failReply marks the reply a sent email carried as failed, when the
address it failed for was the reply's recipient.
*/
func (run reportRun) failReply(ctx context.Context, address, why string) error {
	if run.sent.MessageID == nil || !slices.Contains(run.sent.ToAddresses, address) {
		return nil
	}

	err := run.q.FailEmailMessageDelivery(ctx, models.FailEmailMessageDeliveryParams{
		DeliveryError: new("Email to " + address + " bounced: " + why), Now: run.a.Clock.Now(),
		WorkspaceID: run.sent.WorkspaceID, ID: *run.sent.MessageID,
	})
	if err != nil {
		return fmt.Errorf("fail message delivery: %w", err)
	}

	return nil
}

/*
noticeBounce records a mail system's notice that has no readable
report on the sent email it quotes. The addresses it names as failed
are marked bounced, or every direct recipient when it names none.
*/
func (a *ProcessInboundEmail) noticeBounce(ctx context.Context, mail *received) error {
	reason := ignoreBounceUnmatched

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		for _, original := range quotedIDs(mail) {
			sent, ok, err := findSent(ctx, q, mail.row.WorkspaceID, original)
			if err != nil {
				return err
			}

			if ok {
				reason = ignoreBounce

				return reportRun{a: a, q: q, sent: sent}.noticeFailed(ctx, failedNamed(mail, sent))
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return a.ignore(ctx, mail.row, reason)
}

/*
noticeFailed records each address a notice reported as failed.
*/
func (run reportRun) noticeFailed(ctx context.Context, addresses []string) error {
	bounced := string(enums.OutboundEmailStatusBounced)

	for _, address := range addresses {
		status := recipientStatus{Status: bounced, Diagnostic: noticeDiagnostic, At: run.a.Clock.Now()}
		if err := run.record(ctx, outcome{address: address, status: status, overall: bounced}); err != nil {
			return err
		}

		if err := run.failReply(ctx, address, noticeDiagnostic); err != nil {
			return err
		}
	}

	return nil
}

/*
quotedIDs returns the Message-IDs a notice refers to: the ones it
replies to, then any it quotes from the failed mail.
*/
func quotedIDs(mail *received) []string {
	out := slices.Concat(mail.parsed.InReplyTo, mail.parsed.References)
	for _, match := range quotedMessageID.FindAllStringSubmatch(mail.parsed.Text, -1) {
		out = append(out, match[1])
	}

	return slices.Compact(out)
}

/*
failedNamed returns the sent email's recipients the notice names as
failed, or its direct recipients when it names none of them.
*/
func failedNamed(mail *received, sent models.OutboundEmail) []string {
	var named []string

	for field := range strings.SplitSeq(mail.parsed.Header.Get("X-Failed-Recipients"), ",") {
		address := lower(strings.TrimSpace(field))
		if sentTo(sent, address) {
			named = append(named, address)
		}
	}

	if len(named) == 0 {
		return sent.ToAddresses
	}

	return named
}

/*
complainers returns the sent email's recipients a complaint names, or
all of them when it names none.
*/
func complainers(named []string, sent models.OutboundEmail) []string {
	everyone := slices.Concat(sent.ToAddresses, sent.CcAddresses)

	var out []string

	for _, address := range named {
		if address = lower(address); sentTo(sent, address) {
			out = append(out, address)
		}
	}

	if len(out) == 0 {
		return everyone
	}

	return out
}

/*
sentTo reports whether the sent email went to the address.
*/
func sentTo(sent models.OutboundEmail, address string) bool {
	return slices.Contains(sent.ToAddresses, address) || slices.Contains(sent.CcAddresses, address)
}

/*
testBounced fails the forwarding test whose mail bounced, with what the
bounce says, and suppresses nothing.
*/
func (run reportRun) testBounced(ctx context.Context, address string, status recipientStatus) error {
	err := run.record(ctx, outcome{address: address, status: status, overall: status.Status})
	if err != nil {
		return err
	}

	test, err := run.q.FindEmailAddressTestByOutbound(ctx, models.FindEmailAddressTestByOutboundParams{
		WorkspaceID: run.sent.WorkspaceID, OutboundEmailID: &run.sent.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find bounced email address test: %w", err)
	}

	steps := testSteps{Clock: run.a.Clock, Events: run.a.Events}
	_, err = steps.settle(ctx, run.tx, test, testEnd{
		status: TestFailed, findings: bounceFindings(status.Code, status.Diagnostic),
	})

	return err
}

/*
record stores a recipient's outcome on the sent email, and suppresses
the address when the outcome calls for it.
*/
func (run reportRun) record(ctx context.Context, o outcome) error {
	if o.reason != "" {
		err := run.q.SuppressEmail(ctx, models.SuppressEmailParams{
			WorkspaceID: run.sent.WorkspaceID, Email: o.address, Reason: string(o.reason),
			Scope: string(enums.EmailSuppressionScopeAll), Source: o.source,
			Details:         jsonOf(map[string]string{"code": o.status.Code, "diagnostic": o.status.Diagnostic}),
			OutboundEmailID: &run.sent.ID, ExpiresAt: o.expires, Now: run.a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("suppress email: %w", err)
		}
	}

	_, err := run.q.RecordOutboundRecipientStatus(ctx, models.RecordOutboundRecipientStatusParams{
		Recipient: o.address, RecipientStatus: jsonOf(o.status), Status: o.overall, Now: run.a.Clock.Now(),
		WorkspaceID: run.sent.WorkspaceID, ID: run.sent.ID,
	})
	if err != nil {
		return fmt.Errorf("record recipient status: %w", err)
	}

	return run.broadcastFailed(ctx, o)
}
