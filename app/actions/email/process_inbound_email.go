/*
|--------------------------------------------------------------------------
| Process Inbound Email
|--------------------------------------------------------------------------
|
| The real work on a received email, run by a job after the provider has
| its answer. The raw message is parsed, its sender authenticated, and
| the mail sorted. Mail that is ours coming back, a provider's notice,
| a bounce or a complaint never becomes a message; everything else is
| threaded into its conversation and handed to the inbox.
|
| Every step can run again. A suspended workspace's mail waits as it was
| received. A failure is recorded on the row without any of the mail's
| content and the job tries again; the row says processed, ignored or
| quarantined once a run completes, and a completed row is never
| processed twice.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/arf"
	"github.com/dimmihq/dimmi/framework/mailbox/dsn"
	"github.com/dimmihq/dimmi/framework/mailbox/parse"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
ignoreLoop is the ignore reason of the workspace's own mail coming
back.
*/
const ignoreLoop = "loop_own_outbound"

/*
received is one email being processed: its row, the address it reached,
the raw message and what parsing found.
*/
type received struct {
	row      models.InboundEmail
	address  models.EmailAddress
	raw      []byte
	parsed   *parse.Parsed
	verdicts mailbox.Verdicts
	from     string
	feedback *arf.Report
	delivery *dsn.Report
}

/*
ProcessInboundEmail turns a received email into a message, or records
why it did not become one.

Rules:
  - A suspended workspace's mail waits, received.
  - A row already processed, ignored or quarantined is left alone.
  - Loops, notices, bounces and complaints never become messages.

Errors: a wrapped error reading the raw message, writing rows, or from
the inbox; the row then records it.
*/
type ProcessInboundEmail struct {
	DB       *database.DB
	Clock    clock.Clock
	Storage  *filesystem.Manager
	DNS      *dns.Client
	Limiter  *ratelimit.Limiter
	Config   appconfig.EmailConfig
	Stamps   inbound.Stamps
	Receiver contracts.InboundReceiver
}

/*
Handle processes the received email.
*/
func (a *ProcessInboundEmail) Handle(ctx context.Context, workspaceID, inboundEmailID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())

	mail, ok, err := a.start(ctx, workspaceID, inboundEmailID)
	if err != nil || !ok {
		return err
	}

	if err := a.process(ctx, mail); err != nil {
		a.fail(ctx, mail.row, err)

		return err
	}

	return nil
}

/*
start claims the row and loads what processing needs. It reports false
when there is nothing to do.
*/
func (a *ProcessInboundEmail) start(ctx context.Context, workspaceID, id ids.UUID) (*received, bool, error) {
	mail := &received{}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		ws, err := q.FindWorkspace(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("find workspace: %w", err)
		}

		row, err := q.FindInboundEmail(ctx, models.FindInboundEmailParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			return fmt.Errorf("find inbound email: %w", err)
		}

		if ws.SuspendedAt != nil || ws.DeletedAt != nil || finished(row.Status) || row.EmailAddressID == nil {
			return nil
		}

		mail.address, err = q.FindEmailAddressByID(ctx, models.FindEmailAddressByIDParams{
			WorkspaceID: workspaceID, ID: *row.EmailAddressID,
		})
		if err != nil {
			return fmt.Errorf("find email address: %w", err)
		}

		mail.row, err = q.StartInboundEmailProcessing(ctx, models.StartInboundEmailProcessingParams{
			Now: a.Clock.Now(), WorkspaceID: workspaceID, ID: id,
		})

		return err
	})
	if err != nil || mail.row.ID == ids.Nil() {
		return nil, false, err
	}

	if err := a.load(ctx, mail); err != nil {
		a.fail(ctx, mail.row, err)

		return nil, false, err
	}

	return mail, true, nil
}

/*
load reads and parses the raw message.
*/
func (a *ProcessInboundEmail) load(ctx context.Context, mail *received) error {
	if mail.row.RawPath == nil {
		return errors.New("raw email unavailable")
	}

	disk, err := a.Storage.Disk(mail.row.RawDisk)
	if err != nil {
		return fmt.Errorf("open raw disk: %w", err)
	}

	mail.raw, err = disk.Get(ctx, *mail.row.RawPath)
	if err != nil {
		return fmt.Errorf("read raw email: %w", err)
	}

	mail.parsed, err = parse.Message(bytesReader(mail.raw), parse.Limits{MaxBytes: a.Config.Inbound.MaxMessageBytes})
	if err != nil {
		return fmt.Errorf("parse email: %w", err)
	}

	mail.verdicts = readVerdicts(mail.row.ProviderVerdicts)
	mail.feedback, _ = arf.Parse(mail.raw)
	mail.delivery, _ = dsn.Parse(mail.raw)
	mail.from = senderAddress(mail)

	return nil
}

/*
senderAddress returns the address to answer. When no header field holds
one, the mail is still somebody's: the envelope sender, then Reply-To,
say who. Such a sender is never proven, since mail authentication reads
the From field alone.
*/
func senderAddress(mail *received) string {
	if mail.parsed.From.Email != "" {
		return lower(mail.parsed.From.Email)
	}

	if mail.row.EnvelopeFrom != nil {
		if envelope := lower(strings.Trim(*mail.row.EnvelopeFrom, "<> ")); strings.Contains(envelope, "@") {
			return envelope
		}
	}

	return first(addressesOf(mail.parsed.ReplyTo))
}

/*
process sorts the email and acts on its kind.
*/
func (a *ProcessInboundEmail) process(ctx context.Context, mail *received) error {
	insp, err := a.inspect(ctx, mail)
	if err != nil {
		return err
	}

	if err := a.describe(ctx, mail, insp); err != nil {
		return err
	}

	handlers := []struct {
		class  enums.EmailClassification
		handle func() error
	}{
		{enums.EmailClassificationLoop, func() error { return a.ignore(ctx, mail.row, ignoreLoop) }},
		{enums.EmailClassificationSystemNotice, func() error { return a.notice(ctx, mail, insp) }},
		{enums.EmailClassificationComplaint, func() error { return a.complaint(ctx, mail) }},
		{enums.EmailClassificationBounce, func() error { return a.bounce(ctx, mail) }},
	}

	for _, h := range handlers {
		if h.class == insp.class {
			return h.handle()
		}
	}

	return a.ingest(ctx, mail, insp)
}

/*
describe records what parsing and checking learned on the row.
*/
func (a *ProcessInboundEmail) describe(ctx context.Context, mail *received, insp inspection) error {
	auth := jsonOf(insp.auth)

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		return q.DescribeInboundEmail(ctx, models.DescribeInboundEmailParams{
			MessageIDHeader: optional(mail.parsed.MessageID), FromAddress: optional(mail.from),
			Subject: optional(inbound.DisplaySubject(mail.parsed.Subject)), Classification: new(string(insp.class)),
			Auth: auth, Now: a.Clock.Now(), WorkspaceID: mail.row.WorkspaceID, ID: mail.row.ID,
		})
	})
	if err != nil {
		return fmt.Errorf("describe inbound email: %w", err)
	}

	return nil
}

/*
ignore records why the email became no message.
*/
func (a *ProcessInboundEmail) ignore(ctx context.Context, row models.InboundEmail, reason string) error {
	return within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		return a.ignoreTx(ctx, q, row, reason)
	})
}

/*
fail records why processing failed. The error says what went wrong,
never what the mail said.
*/
func (a *ProcessInboundEmail) fail(ctx context.Context, row models.InboundEmail, cause error) {
	message := diagnostic(cause.Error())

	_ = within(context.WithoutCancel(ctx), a.DB, func(ctx context.Context, q *models.Queries) error {
		return q.FailInboundEmail(ctx, models.FailInboundEmailParams{
			Error: &message, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})
}

/*
finished reports whether a row's processing already completed.
*/
func finished(status string) bool {
	return slices.Contains([]enums.InboundEmailStatus{
		enums.InboundEmailStatusProcessed, enums.InboundEmailStatusIgnored, enums.InboundEmailStatusQuarantined,
	}, enums.InboundEmailStatus(status))
}
