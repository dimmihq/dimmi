/*
|--------------------------------------------------------------------------
| Inspecting Received Mail
|--------------------------------------------------------------------------
|
| Everything known about a received email before it is threaded: who
| really sent it, whether that is proven, what kind of mail it is, and
| what the customer wrote. Authentication never blocks on DNS: a lookup
| that fails leaves the result unknown and the mail moves on.
|
| A sender who mails a workspace too often in a short time is still
| heard, but nothing answers them automatically; far too often, and the
| mail is set aside as spam. The counters live in Valkey and only ever
| loosen when lost.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/authcheck"

	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
inspection is what was learned about a received email.

Member is the teammate who sent it, authenticated, when one did.
Forward is the message a teammate forwarded. Relay names the form
service that sent it for a visitor. Confirmation is a provider's
forwarding confirmation, ForwardingTest a forwarding test of the
address. Throttled means the sender wrote too often to be answered
automatically. Own lists the workspace's own addresses.
*/
type inspection struct {
	class          enums.EmailClassification
	auth           inbound.Auth
	body           inbound.Body
	member         *models.FindEmailSenderMemberRow
	forward        *inbound.Forward
	relay          string
	confirmation   *inbound.Confirmation
	forwardingTest bool
	throttled      bool
	own            []string
}

/*
facts are what the workspace's own records say about a received email.
*/
type facts struct {
	own          []string
	outboundRefs int64
	member       *models.FindEmailSenderMemberRow
}

/*
inspect learns everything about the email that sorting needs, and
sorts it.
*/
func (a *ProcessInboundEmail) inspect(ctx context.Context, mail *received) (inspection, error) {
	body, err := inbound.BuildBody(mail.parsed.HTML, mail.parsed.Text)
	if err != nil {
		return inspection{}, fmt.Errorf("build body: %w", err)
	}

	known, err := a.facts(ctx, mail)
	if err != nil {
		return inspection{}, err
	}

	insp := inspection{auth: a.authenticate(ctx, mail), body: body, own: known.own}
	if known.member != nil && insp.auth.Passed() {
		insp.member = known.member
		if forward, ok := inbound.FindForward(mail.parsed.Text); ok {
			insp.forward = &forward
		}
	}

	insp.relay, _ = inbound.FormRelay(mail.from, addressesOf(mail.parsed.ReplyTo))
	if confirmation, ok := inbound.FindConfirmation(mail.from, mail.parsed.Subject, mail.parsed.Text+"\n"+
		mail.parsed.HTML); ok {
		insp.confirmation = &confirmation
	}

	insp.forwardingTest = a.Stamps.ValidTest(mail.parsed.Header.Get(inbound.TestHeader), mail.address.ID.String())

	throttled, quarantined := a.rate(ctx, mail)
	insp.throttled = throttled

	insp.class = inbound.Classify(inbound.Signals{
		Header: mail.parsed.Header, Subject: mail.parsed.Subject, From: mail.from,
		VisibleChars: len(body.Markdown),
		Loop: a.Stamps.ValidLoop(mail.parsed.Header.Get(inbound.LoopHeader), mail.row.WorkspaceID.String(),
			mail.parsed.MessageID) || known.outboundRefs > 0,
		SystemNotice: insp.confirmation != nil || insp.forwardingTest,
		Complaint:    isComplaint(mail),
		Bounce:       isBounce(mail),
		Spam:         spam(mail.verdicts) || quarantined,
		Member:       insp.member != nil, Forwarded: insp.forward != nil, FormRelay: insp.relay != "",
	})

	return insp, nil
}

/*
facts reads the workspace's own addresses, whether the email is mail
the workspace sent, and which teammate sent it.
*/
func (a *ProcessInboundEmail) facts(ctx context.Context, mail *received) (facts, error) {
	var known facts

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		ws := mail.row.WorkspaceID

		var err error

		known.own, err = q.ListWorkspaceEmailIdentities(ctx, ws)
		if err != nil {
			return fmt.Errorf("list workspace addresses: %w", err)
		}

		if mail.parsed.MessageID != "" {
			known.outboundRefs, err = q.CountOutboundRefs(ctx, models.CountOutboundRefsParams{
				WorkspaceID: ws, RfcMessageIds: []string{mail.parsed.MessageID},
			})
			if err != nil {
				return fmt.Errorf("count outbound refs: %w", err)
			}
		}

		member, err := q.FindEmailSenderMember(ctx, models.FindEmailSenderMemberParams{
			WorkspaceID: ws, Email: mail.from,
		})
		if err == nil {
			known.member = &member
		}

		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find sender member: %w", err)
		}

		return nil
	})

	return known, err
}

/*
authenticate verifies DKIM and DMARC alignment here and takes SPF from
the provider's verdict.
*/
func (a *ProcessInboundEmail) authenticate(ctx context.Context, mail *received) inbound.Auth {
	envelopeFrom := ""
	if mail.row.EnvelopeFrom != nil {
		envelopeFrom = *mail.row.EnvelopeFrom
	}

	results := authcheck.New(a.DNS).Verify(ctx, authcheck.Input{
		Raw: mail.raw, MailFrom: envelopeFrom, SPF: inbound.SPFVerdict(mail.verdicts.SPF),
	})

	return inbound.Evaluate(results, mail.verdicts, mail.row.Source)
}

/*
rate counts the email against its sender's limits and reports whether
the sender is throttled or should be quarantined. Without Valkey both
limits are open.
*/
func (a *ProcessInboundEmail) rate(ctx context.Context, mail *received) (bool, bool) {
	limits := a.Config.Inbound
	key := "email:sender:" + mail.row.WorkspaceID.String() + ":" + hashing.SHA256Hex([]byte(mail.from))

	throttle, err := a.Limiter.Hit(ctx, key+":rate", limits.SenderRate, limits.SenderRateWindow)
	if err != nil {
		return false, false
	}

	quarantine, err := a.Limiter.Hit(ctx, key+":quarantine", limits.SenderQuarantine, limits.SenderQuarantineWindow)
	if err != nil {
		return !throttle.Allowed, false
	}

	return !throttle.Allowed, !quarantine.Allowed
}

/*
isComplaint reports whether the email is a feedback report.
*/
func isComplaint(mail *received) bool {
	return mail.feedback != nil
}

/*
isBounce reports whether the email is a delivery report, or reads as a
bounce without one.
*/
func isBounce(mail *received) bool {
	if mail.delivery != nil {
		return true
	}

	envelopeFrom := ""
	if mail.row.EnvelopeFrom != nil {
		envelopeFrom = *mail.row.EnvelopeFrom
	}

	return inbound.BounceNotice(envelopeFrom, mail.from, mail.parsed.Subject)
}

/*
spam reports whether the provider condemned the email: spam, a virus,
or a sender domain that rejects mail failing DMARC.
*/
func spam(v mailbox.Verdicts) bool {
	return v.Spam == mailbox.VerdictFail || v.Virus == mailbox.VerdictFail ||
		(v.DMARC == mailbox.VerdictFail && strings.EqualFold(v.DMARCPolicy, "reject"))
}
