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
*/

package email

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/mailbox/arf"
	"github.com/dimmihq/dimmi/framework/mailbox/dsn"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

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
	report, err := arf.Parse(mail.raw)
	if err != nil {
		return fmt.Errorf("parse feedback report: %w", err)
	}

	if !report.Complaint() {
		return a.ignore(ctx, mail.row, ignoreComplaintNotSpam)
	}

	reason := ignoreComplaint + unmatchedSuffix

	err = within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
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
	report, err := dsn.Parse(mail.raw)
	if err != nil {
		return a.ignore(ctx, mail.row, ignoreBounceUnmatched)
	}

	original := report.OriginalMessageID
	if original == "" && len(mail.parsed.InReplyTo) > 0 {
		original = mail.parsed.InReplyTo[0]
	}

	reason := ignoreBounceUnmatched

	err = within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
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
	if err != nil || run.sent.MessageID == nil || !slices.Contains(run.sent.ToAddresses, address) {
		return err
	}

	err = run.q.FailEmailMessageDelivery(ctx, models.FailEmailMessageDeliveryParams{
		DeliveryError: new("Email to " + address + " bounced: " + status.Diagnostic), Now: run.a.Clock.Now(),
		WorkspaceID: run.sent.WorkspaceID, ID: *run.sent.MessageID,
	})
	if err != nil {
		return fmt.Errorf("fail message delivery: %w", err)
	}

	return nil
}

/*
record stores a recipient's outcome on the sent email, and suppresses
the address when the outcome calls for it.
*/
func (run reportRun) record(ctx context.Context, o outcome) error {
	if o.reason != "" {
		details, err := jsonOf(map[string]string{"code": o.status.Code, "diagnostic": o.status.Diagnostic})
		if err != nil {
			return err
		}

		err = run.q.SuppressEmail(ctx, models.SuppressEmailParams{
			WorkspaceID: run.sent.WorkspaceID, Email: o.address, Reason: string(o.reason),
			Scope: string(enums.EmailSuppressionScopeAll), Source: o.source, Details: details,
			OutboundEmailID: &run.sent.ID, Now: run.a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("suppress email: %w", err)
		}
	}

	encoded, err := jsonOf(o.status)
	if err != nil {
		return err
	}

	_, err = run.q.RecordOutboundRecipientStatus(ctx, models.RecordOutboundRecipientStatusParams{
		Recipient: o.address, RecipientStatus: encoded, Status: o.overall, Now: run.a.Clock.Now(),
		WorkspaceID: run.sent.WorkspaceID, ID: run.sent.ID,
	})
	if err != nil {
		return fmt.Errorf("record recipient status: %w", err)
	}

	return nil
}
