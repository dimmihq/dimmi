/*
|--------------------------------------------------------------------------
| Messaging Everyone Linked
|--------------------------------------------------------------------------
|
| A tracker or incident writes once to every linked customer. A dry run
| counts who it reaches and who would be held back, with the reason: the
| customer wrote since they were linked and nobody answered, or a
| teammate already wrote to them after the last update. Sending stores
| the message with one delivery row per conversation and queues a job,
| which sends each in the customer's own channel, may resolve their
| ticket or close their conversation, and records how each went, so a
| partial failure shows exactly who did not hear.
|
*/

package tickets

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The reasons a recipient is held back.
*/
const (
	HeldCustomerWrote = "customer_wrote_since"
	HeldTeamTold      = "teammate_already_replied"
)

/*
UpdateInput is a message to everyone linked. Resolve closes their
conversations and resolves their tickets after it is sent. DryRun only
counts.
*/
type UpdateInput struct {
	Body    string
	Resolve bool
	DryRun  bool
}

/*
Recipient is one conversation an update reaches, held or not.
*/
type Recipient struct {
	ConversationID ids.UUID
	Held           string
}

/*
Plan is who an update reaches: the recipients and the update stored,
none on a dry run.
*/
type Plan struct {
	Recipients []Recipient
	Update     *models.TicketUpdate
}

/*
SendToLinked plans an update to a tracker's linked customers and, unless
it is a dry run, queues it.
*/
func (t *Tickets) SendToLinked(ctx context.Context, m authz.Membership, ref Ref, in UpdateInput) (Plan, error) {
	if err := canWrite(m); err != nil {
		return Plan{}, err
	}

	in.Body = strings.TrimSpace(in.Body)
	if n := len([]rune(in.Body)); n < 1 || n > 20000 {
		return Plan{}, ErrNoteInvalid
	}

	var out Plan

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		if found.Kind != KindTracker && found.Kind != KindIncident {
			return ErrNotTracker
		}

		out, err = w.planUpdate(ctx, found, in)

		return err
	})

	return out, err
}

/*
planUpdate finds the recipients and, unless dry, stores the update and
queues its job.
*/
func (w *Writer) planUpdate(ctx context.Context, t models.Ticket, in UpdateInput) (Plan, error) {
	rows, err := w.Q.ListTrackerRecipients(ctx, models.ListTrackerRecipientsParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("list update recipients: %w", err)
	}

	last := w.lastUpdate(ctx, t)
	out := Plan{Recipients: make([]Recipient, len(rows))}
	for i, r := range rows {
		out.Recipients[i] = Recipient{ConversationID: r.ID, Held: holdReason(r, last)}
	}

	if in.DryRun {
		return out, nil
	}

	update, err := w.storeUpdate(ctx, t, in, out.Recipients)
	if err != nil {
		return out, err
	}

	out.Update = &update

	return out, nil
}

/*
lastUpdate returns when the tracker last messaged its customers.
*/
func (w *Writer) lastUpdate(ctx context.Context, t models.Ticket) *time.Time {
	list, err := w.Q.ListTicketUpdates(ctx, models.ListTicketUpdatesParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})
	if err != nil || len(list) == 0 {
		return nil
	}

	return &list[0].CreatedAt
}

/*
holdReason says why a recipient should not get the update, empty when
it should: a customer who wrote after being linked and got no answer
needs a person, and one a teammate wrote to after the last update was
told already.
*/
func holdReason(r models.ListTrackerRecipientsRow, last *time.Time) string {
	wrote := r.LastCustomerMessageAt != nil && r.LastCustomerMessageAt.After(r.LinkedAt)
	answered := r.LastReplyAt != nil && r.LastCustomerMessageAt != nil && r.LastReplyAt.After(*r.LastCustomerMessageAt)

	switch {
	case wrote && !answered:
		return HeldCustomerWrote
	case last != nil && r.LastReplyAt != nil && r.LastReplyAt.After(*last):
		return HeldTeamTold
	}

	return ""
}

/*
storeUpdate stores the update and its deliveries and queues the send.
*/
func (w *Writer) storeUpdate(
	ctx context.Context, t models.Ticket, in UpdateInput, recipients []Recipient,
) (models.TicketUpdate, error) {
	held := 0
	for _, r := range recipients {
		if r.Held != "" {
			held++
		}
	}

	var author *ids.UUID
	if w.Actor.Type == string(enums.AuthorTypeMember) {
		author = w.actorID()
	}

	u, err := w.Q.CreateTicketUpdate(ctx, models.CreateTicketUpdateParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, Body: in.Body, Resolve: in.Resolve,
		Recipients: columnCount(len(recipients)), Held: columnCount(held), AuthorMemberID: author, Now: w.Now,
	})
	if err != nil {
		return u, fmt.Errorf("store ticket update: %w", err)
	}

	for _, r := range recipients {
		status := "pending"
		if r.Held != "" {
			status = "held"
		}

		if err := w.Q.CreateTicketUpdateDelivery(ctx, models.CreateTicketUpdateDeliveryParams{
			WorkspaceID: t.WorkspaceID, UpdateID: u.ID, ConversationID: r.ConversationID, Status: status,
			Reason: nonEmpty(r.Held), Now: w.Now,
		}); err != nil {
			return u, fmt.Errorf("store update recipient: %w", err)
		}
	}

	if err := w.log(ctx, t, "update_queued", map[string]any{
		"update_id": models.PublicID(models.PrefixTicketUpdate, u.ID), "recipients": len(recipients), "held": held,
	}); err != nil {
		return u, err
	}

	job := jobs.SendTicketUpdateArgs{WorkspaceID: t.WorkspaceID, UpdateID: u.ID}
	if err := w.Owner.Queue.PushTx(ctx, w.Tx, job); err != nil {
		return u, fmt.Errorf("queue ticket update: %w", err)
	}

	return u, nil
}

/*
sendBatch is how many recipients one transaction sends to.
*/
const sendBatch = 50

/*
SendUpdate sends a queued update to its pending recipients, a batch per
transaction, and records how it went.
*/
func (t *Tickets) SendUpdate(ctx context.Context, args jobs.SendTicketUpdateArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	for {
		more, err := t.sendBatch(ctx, args)
		if err != nil {
			return err
		}

		if !more {
			return t.finishUpdate(ctx, args)
		}
	}
}

/*
sendBatch sends to one batch of pending recipients and reports whether
more wait.
*/
func (t *Tickets) sendBatch(ctx context.Context, args jobs.SendTicketUpdateArgs) (bool, error) {
	var more bool

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, args.WorkspaceID)
		if err != nil {
			return err
		}

		u, err := w.Q.FindTicketUpdate(ctx, models.FindTicketUpdateParams{
			WorkspaceID: args.WorkspaceID, ID: args.UpdateID,
		})
		if err != nil {
			return nil //nolint:nilerr // an update gone has nothing left to send
		}

		_ = w.Q.MarkTicketUpdateSending(ctx, models.MarkTicketUpdateSendingParams{
			WorkspaceID: args.WorkspaceID, ID: u.ID,
		})

		pending, err := w.Q.ListPendingTicketUpdateDeliveries(ctx, models.ListPendingTicketUpdateDeliveriesParams{
			WorkspaceID: args.WorkspaceID, UpdateID: u.ID, RowLimit: sendBatch,
		})
		if err != nil {
			return fmt.Errorf("list pending recipients: %w", err)
		}

		for _, d := range pending {
			if err := t.deliver(ctx, w, u, d); err != nil {
				return err
			}
		}

		more = len(pending) == sendBatch

		return nil
	})

	return more, err
}

/*
deliver sends the update to one recipient, inside a savepoint so one
failure only marks that recipient.
*/
func (t *Tickets) deliver(ctx context.Context, w *Writer, u models.TicketUpdate, d models.TicketUpdateDelivery) error {
	var sent *ids.UUID

	err := w.Tx.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		var err error
		if sent, err = t.send(ctx, w.Writer, u.WorkspaceID, sendInput{
			conversation: d.ConversationID, body: u.Body,
			dedupe: "ticket-update:" + u.ID.String() + ":" + d.ID.String(),
		}); err != nil {
			return err
		}

		if u.Resolve && sent != nil {
			return t.resolveRecipient(ctx, w, u.WorkspaceID, d.ConversationID)
		}

		return nil
	})

	status, reason := "sent", ""
	switch {
	case err != nil:
		status, reason = "failed", cutText(err.Error(), 200)
	case sent == nil:
		status, reason = "failed", "customer_unreachable"
	}

	params := models.SetTicketUpdateDeliveryParams{
		Status: status, Reason: nonEmpty(reason), MessageID: sent, Now: w.Now, WorkspaceID: u.WorkspaceID, ID: d.ID,
	}
	if status == "sent" {
		params.SentAt = &w.Now
	}

	if err := w.Q.SetTicketUpdateDelivery(ctx, params); err != nil {
		return fmt.Errorf("record update delivery: %w", err)
	}

	return nil
}

/*
resolveRecipient resolves the recipient's own ticket, or closes the
conversation when it has none.
*/
func (t *Tickets) resolveRecipient(ctx context.Context, w *Writer, ws, conversationID ids.UUID) error {
	c, err := w.Q.LockConversation(ctx, models.LockConversationParams{WorkspaceID: ws, ID: conversationID})
	if err != nil {
		return fmt.Errorf("find recipient conversation: %w", err)
	}

	own, err := w.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if err == nil && own.DeletedAt == nil && own.StateGroup != GroupResolved {
		if st, ok := w.Setup.DefaultResolved(own.TypeID); ok {
			_, err := w.move(ctx, own, st)

			return err
		}
	}

	_, _, err = w.ChangeStatus(ctx, c, conversations.StatusChange{
		Status: string(enums.ConversationStatusClosed), Reason: conversations.ReasonManual,
	})

	return err
}

/*
finishUpdate counts the deliveries, marks the update done and announces
it.
*/
func (t *Tickets) finishUpdate(ctx context.Context, args jobs.SendTicketUpdateArgs) error {
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, args.WorkspaceID)
		if err != nil {
			return err
		}

		u, err := w.Q.FinishTicketUpdate(ctx, models.FinishTicketUpdateParams{
			Status: "sent", Now: &w.Now, WorkspaceID: args.WorkspaceID, ID: args.UpdateID,
		})
		if err != nil {
			return nil //nolint:nilerr // an update gone has nothing to finish
		}

		if u.Failed > 0 {
			_, _ = w.Q.FinishTicketUpdate(ctx, models.FinishTicketUpdateParams{
				Status: finalStatus(u), Now: &w.Now, WorkspaceID: args.WorkspaceID, ID: args.UpdateID,
			})
		}

		ticket, err := w.Q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: args.WorkspaceID, ID: u.TicketID})
		if err != nil {
			return notFound(err)
		}

		counts := map[string]any{
			"delivered": u.Delivered, "held": u.Held, "failed": u.Failed,
			"update_id": models.PublicID(models.PrefixTicketUpdate, u.ID),
		}
		if err := w.log(ctx, ticket, "update_sent", counts); err != nil {
			return err
		}

		changes := map[string]appevents.TicketChange{"update": change(nil, counts)}

		return w.emit(ctx, appevents.TicketUpdateSent{TicketChanged: w.changed(ticket, changes)})
	})
}

/*
finalStatus says how an update with failures ended.
*/
func finalStatus(u models.TicketUpdate) string {
	if u.Delivered == 0 {
		return "failed"
	}

	return "partial"
}

/*
cutText cuts text to a length.
*/
func cutText(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}

	return s
}

/*
UpdateReport is a sent update with its deliveries.
*/
type UpdateReport struct {
	Update     models.TicketUpdate
	Deliveries []models.ListTicketUpdateDeliveriesRow
}

/*
Updates returns a tracker's updates with each recipient's result.
*/
func (t *Tickets) Updates(ctx context.Context, m authz.Membership, ref Ref) ([]UpdateReport, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []UpdateReport

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		updates, err := q.ListTicketUpdates(ctx, models.ListTicketUpdatesParams{
			WorkspaceID: m.Workspace.ID, TicketID: found.ID,
		})
		if err != nil {
			return fmt.Errorf("list ticket updates: %w", err)
		}

		out, err = withDeliveries(ctx, q, m.Workspace.ID, updates)

		return err
	})

	return out, err
}

/*
withDeliveries adds each update's recipients.
*/
func withDeliveries(
	ctx context.Context, q *models.Queries, ws ids.UUID, updates []models.TicketUpdate,
) ([]UpdateReport, error) {
	updateIDs := make([]ids.UUID, len(updates))
	for i, u := range updates {
		updateIDs[i] = u.ID
	}

	rows, err := q.ListTicketUpdateDeliveries(ctx, models.ListTicketUpdateDeliveriesParams{
		WorkspaceID: ws, UpdateIds: updateIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list update deliveries: %w", err)
	}

	out := make([]UpdateReport, len(updates))
	for i, u := range updates {
		out[i].Update = u
		for _, r := range rows {
			if r.UpdateID == u.ID {
				out[i].Deliveries = append(out[i].Deliveries, r)
			}
		}
	}

	return out, nil
}

/*
columnCount writes a count as the column's size, capped at its largest.
*/
func columnCount(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n) //nolint:gosec // capped above
}
