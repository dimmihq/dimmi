/*
|--------------------------------------------------------------------------
| Receive Inbound Email
|--------------------------------------------------------------------------
|
| The fast first step every inbound source ends in, run while the
| provider waits for an answer. It finds whose forwarding addresses the
| mail was sent to, keeps the raw message, records one row per
| workspace and queues the real work, all before answering.
|
| Each workspace only ever learns about its own addresses: mail to two
| workspaces becomes two rows that never name each other. Mail to an
| address nobody holds is dropped without a trace in any workspace.
| Mail to an address deleted within the last 30 days is still recorded,
| ignored, so an admin sees that a forward is still set up somewhere. A
| provider delivering the same message twice records it once.
|
*/

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
deletedAddressGrace is how long mail to a deleted address is still
recorded for the admin.
*/
const deletedAddressGrace = 30 * 24 * time.Hour

/*
ignoreAddressDeleted is the ignore reason of mail to a deleted address.
*/
const ignoreAddressDeleted = "address_deleted"

/*
ReceivedEmail is one workspace's record of a received message.
Duplicate means the provider delivered it before.
*/
type ReceivedEmail struct {
	WorkspaceID    ids.UUID
	InboundEmailID ids.UUID
	Duplicate      bool
}

/*
ReceiveInboundEmail records a received message for every workspace it
was sent to and queues its processing.

Rules:
  - Only recipients on the inbound domain that route to an address
    count.
  - The raw message is kept once, on the raw disk.
  - A workspace's row lists only that workspace's addresses.

Errors: a wrapped error reading or storing the raw message or writing
the rows.
*/
type ReceiveInboundEmail struct {
	DB       *database.DB
	Clock    clock.Clock
	Queue    *queue.Queue
	Disk     filesystem.Filesystem
	DiskName string
	Config   appconfig.EmailConfig
}

/*
route is one workspace's share of a message: its addresses among the
recipients and the address the message belongs to.
*/
type route struct {
	workspaceID    ids.UUID
	addressID      ids.UUID
	connectionID   ids.UUID
	recipients     []string
	deleted        bool
	tokenAddressed bool
}

/*
Handle records the message and returns one record per workspace.
*/
func (a *ReceiveInboundEmail) Handle(ctx context.Context, env mailbox.Envelope) ([]ReceivedEmail, error) {
	routes, err := a.route(ctx, env.Recipients)
	if err != nil || len(routes) == 0 {
		return nil, err
	}

	path, size, err := a.store(ctx, env)
	if err != nil {
		return nil, err
	}

	verdicts, err := json.Marshal(env.Verdicts)
	if err != nil {
		return nil, fmt.Errorf("encode verdicts: %w", err)
	}

	received := make([]ReceivedEmail, 0, len(routes))
	for _, r := range routes {
		row, err := a.record(ctx, env, r, recordedRaw{path: path, size: size, verdicts: verdicts})
		if err != nil {
			return received, err
		}

		received = append(received, row)
	}

	return received, nil
}

/*
route groups the recipients by workspace. Within a workspace the
message belongs to the first address reached through a reply token,
else the first address in recipient order.
*/
func (a *ReceiveInboundEmail) route(ctx context.Context, recipients []string) ([]route, error) {
	var routes []route

	q := models.New(a.DB)
	since := a.Clock.Now().Add(-deletedAddressGrace)

	for _, recipient := range recipients {
		parsed, ok := inbound.ParseRecipient(recipient, a.Config.InboundDomain)
		if !ok {
			continue
		}

		found, err := q.RouteEmailAddress(ctx, models.RouteEmailAddressParams{
			DeletedSince: since, ForwardingAddress: parsed.Forwarding,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && found.Deleted && !found.DeletedRecently) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("route %s: %w", parsed.Forwarding, err)
		}

		routes = addRoute(routes, found, parsed, lower(recipient))
	}

	return routes, nil
}

/*
addRoute adds a routed recipient to its workspace's share, letting a
reply token address take the message over from a plain one.
*/
func addRoute(routes []route, found models.RouteEmailAddressRow, parsed inbound.Recipient, address string) []route {
	i := slices.IndexFunc(routes, func(r route) bool { return r.workspaceID == found.WorkspaceID })
	if i < 0 {
		return append(routes, route{
			workspaceID: found.WorkspaceID, addressID: found.EmailAddressID, connectionID: found.ChannelConnectionID,
			recipients: []string{address}, deleted: found.Deleted, tokenAddressed: parsed.Token != "",
		})
	}

	r := &routes[i]
	if !slices.Contains(r.recipients, address) {
		r.recipients = append(r.recipients, address)
	}

	if parsed.Token != "" && !r.tokenAddressed && !found.Deleted {
		r.addressID, r.connectionID, r.deleted, r.tokenAddressed = found.EmailAddressID, found.ChannelConnectionID,
			false, true
	}

	return routes
}

/*
store keeps the raw message on the raw disk and returns its path and
size.
*/
func (a *ReceiveInboundEmail) store(ctx context.Context, env mailbox.Envelope) (string, int64, error) {
	if env.RawRef != nil {
		return env.RawRef.Path, env.SizeBytes, nil
	}

	raw, err := env.OpenRaw(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("open raw email: %w", err)
	}
	defer func() { _ = raw.Close() }()

	content, err := io.ReadAll(io.LimitReader(raw, a.Config.Inbound.MaxMessageBytes*2))
	if err != nil {
		return "", 0, fmt.Errorf("read raw email: %w", err)
	}

	now := a.Clock.Now().UTC()
	path := fmt.Sprintf("inbound-emails/%s/%s.eml", now.Format("2006/01/02"), ids.NewV7At(now))

	err = a.Disk.Put(ctx, path, content, filesystem.PutOptions{ContentType: "message/rfc822"})
	if err != nil {
		return "", 0, fmt.Errorf("store raw email: %w", err)
	}

	return path, int64(len(content)), nil
}

/*
recordedRaw is where the raw message was kept and what the provider
concluded about it.
*/
type recordedRaw struct {
	path     string
	size     int64
	verdicts json.RawMessage
}

/*
record writes the workspace's row and queues its processing, in one
transaction. A row the provider delivered before is left alone.
*/
func (a *ReceiveInboundEmail) record(
	ctx context.Context, env mailbox.Envelope, r route, raw recordedRaw,
) (ReceivedEmail, error) {
	out := ReceivedEmail{WorkspaceID: r.workspaceID}

	params := models.CreateInboundEmailParams{
		WorkspaceID: r.workspaceID, EmailAddressID: &r.addressID, ChannelConnectionID: &r.connectionID,
		Source: env.Source, ProviderMessageID: optional(env.ProviderMessageID),
		EnvelopeFrom: optional(env.EnvelopeFrom), EnvelopeTo: r.recipients, SizeBytes: raw.size,
		RawDisk: a.DiskName, RawPath: &raw.path, Status: string(enums.InboundEmailStatusReceived),
		ProviderVerdicts: raw.verdicts, ReceivedAt: env.ReceivedAt, Now: a.Clock.Now(),
	}

	if r.deleted {
		params.Status, params.IgnoreReason = string(enums.InboundEmailStatusIgnored), new(ignoreAddressDeleted)
		params.ProcessedAt = new(a.Clock.Now())
	}

	err := a.DB.Transaction(inWorkspace(ctx, r.workspaceID), func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).CreateInboundEmail(ctx, params)
		if errors.Is(err, database.ErrNotFound) {
			out.Duplicate = true

			return nil
		}

		if err != nil {
			return fmt.Errorf("record inbound email: %w", err)
		}

		out.InboundEmailID = row.ID
		if r.deleted {
			return nil
		}

		return a.Queue.PushTx(ctx, tx, jobs.ProcessInboundEmailArgs{WorkspaceID: r.workspaceID, InboundEmailID: row.ID})
	})

	return out, err
}
