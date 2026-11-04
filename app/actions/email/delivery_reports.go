/*
|--------------------------------------------------------------------------
| Delivery Reports
|--------------------------------------------------------------------------
|
| Bounces and spam complaints arrive as mail about mail the workspace
| sent. Each is matched to the sent email by its Message-ID. A hard
| bounce stops mail to the address and fails the reply meant for it; a
| delay is only noted. A complaint stops all mail to the complainer.
| A report about nothing the workspace sent is recorded as unmatched.
|
| A mail system's notice without a readable report is still matched,
| through the ids it quotes, and fails the reply it names. It suppresses
| nothing, because nothing in it says the address is gone for good.
|
*/

package email

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

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
noticeDiagnostic explains a failure read from a notice with no report.
*/
const noticeDiagnostic = "the receiving mail system said the email could not be delivered"

/*
outcome is one recipient's fate to record: the address, what became of
it, the suppression it causes, if any, and the status of the whole
email afterwards.
*/
type outcome struct {
	address string
	status  recipientStatus
	reason  enums.EmailSuppressionReason
	source  string
	overall string
}

/*
reportRun records a report's outcomes on one sent email.
*/
type reportRun struct {
	a    *ProcessInboundEmail
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

		for _, recipient := range slices.Concat(sent.ToAddresses, sent.CcAddresses) {
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

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		sent, ok, err := findSent(ctx, q, mail.row.WorkspaceID, original)
		if err != nil || !ok {
			return err
		}

		reason = ignoreBounce
		run := reportRun{a: a, q: q, sent: sent}

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
bounced records one recipient of a delivery report. A hard bounce stops
mail to the address and fails the reply when it was meant for it.
*/
func (run reportRun) bounced(ctx context.Context, r dsn.Recipient) error {
	address := lower(r.FinalRecipient)
	status := recipientStatus{
		Status: "delayed", Code: r.Status, Diagnostic: diagnostic(r.DiagnosticCode), At: run.a.Clock.Now(),
	}

	if !r.Permanent() {
		return run.record(ctx, outcome{address: address, status: status, overall: run.sent.Status})
	}

	status.Status = string(enums.OutboundEmailStatusBounced)

	err := run.record(ctx, outcome{
		address: address, status: status, reason: enums.EmailSuppressionReasonHardBounce, source: "dsn",
		overall: status.Status,
	})
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
		if slices.Contains(sent.ToAddresses, address) || slices.Contains(sent.CcAddresses, address) {
			named = append(named, address)
		}
	}

	if len(named) == 0 {
		return sent.ToAddresses
	}

	return named
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
			OutboundEmailID: &run.sent.ID, Now: run.a.Clock.Now(),
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

	return nil
}
