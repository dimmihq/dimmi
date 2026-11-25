/*
|--------------------------------------------------------------------------
| Conversation Broadcasts
|--------------------------------------------------------------------------
|
| A conversation broadcast answers each person inside a conversation
| they already have, on that conversation's own channel: the email
| thread, the Messenger, Slack, WhatsApp or any other. One broadcast so
| reaches people across every channel at once, and their answers land
| in the thread they already know.
|
| Started from a tracker, it reaches only the conversations linked to
| that tracker. Otherwise each person in the audience is answered in
| their newest conversation. Either way the audience narrows who is
| reached, and each person is answered once. Two holds can leave people
| out: a customer still waiting on an answer, who needs a person rather
| than a broadcast, and, for a tracker, a customer a teammate already
| answered since the tracker last moved.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Holds a conversation broadcast can keep.
*/
const (
	HoldCustomerWaiting = "customer_waiting"
	HoldTeamReplied     = "team_replied"
)

/*
Reasons a conversation broadcast leaves a person out or fails them.
*/
const (
	ReasonCustomerWaiting = "customer_waiting"
	ReasonTeamReplied     = "team_replied"
	ReasonUnreachable     = "unreachable"
	ReasonNoChannel       = "channel_unsupported"
)

/*
validHolds reports whether every hold is one a broadcast can keep.
*/
func validHolds(holds []string) bool {
	for _, h := range holds {
		if h != HoldCustomerWaiting && h != HoldTeamReplied {
			return false
		}
	}

	return true
}

/*
scope is what a conversation broadcast answers in: the tracker when it
has one, and since when a teammate's answer counts as already told.
*/
type scope struct {
	ticket *ids.UUID
	since  *time.Time
}

/*
scopeOf reads the broadcast's tracker, if it has one.
*/
func scopeOf(ctx context.Context, q *models.Queries, row models.Broadcast) (scope, error) {
	if row.TicketID == nil {
		return scope{}, nil
	}

	t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: row.WorkspaceID, ID: *row.TicketID})
	if errors.Is(err, database.ErrNotFound) {
		return scope{ticket: row.TicketID}, nil
	}

	if err != nil {
		return scope{}, fmt.Errorf("find tracker: %w", err)
	}

	return scope{ticket: row.TicketID, since: &t.StateSince}, nil
}

/*
conversationBatch is one page of conversations in scope: the people it
keeps as candidates, the last conversation read, and whether it was the
final page.
*/
type conversationBatch struct {
	people []candidate
	last   ids.UUID
	done   bool
}

/*
conversationCursor is where the next page of conversations in scope
starts: after the conversation last read.
*/
type conversationCursor struct {
	scope scope
	after ids.UUID
}

/*
conversationPage reads one page of the conversations in scope and keeps
those whose customer the audience matches, each as a candidate.
*/
func (b *Broadcasts) conversationPage(
	ctx context.Context, q *models.Queries, row models.Broadcast, at conversationCursor,
) (conversationBatch, error) {
	s, after := at.scope, at.after

	page, err := q.BroadcastConversationPage(ctx, models.BroadcastConversationPageParams{
		WorkspaceID: row.WorkspaceID, TicketID: s.ticket, After: after, PageSize: freezePage,
	})
	if err != nil {
		return conversationBatch{last: after}, wrap("read conversations", err)
	}

	done := len(page) < freezePage
	if len(page) == 0 {
		return conversationBatch{last: after, done: done}, nil
	}

	people := make([]models.Contact, 0, len(page))
	for _, r := range page {
		people = append(people, r.Contact)
	}

	kept, err := b.Audiences.Keep(ctx, q, audiences.KeepInput{
		WorkspaceID: row.WorkspaceID, AudienceID: row.AudienceID, Rules: row.Rules, Contacts: people,
	})
	if err != nil {
		return conversationBatch{last: after}, err
	}

	out := b.candidates(row, s, page, kept)

	return conversationBatch{people: out, last: page[len(page)-1].Conversation.ID, done: done}, nil
}

/*
candidates turns the conversations of a page whose customer the
audience kept into candidates, each skipped when a hold leaves it out.
*/
func (b *Broadcasts) candidates(
	row models.Broadcast, s scope, page []models.BroadcastConversationPageRow, kept []models.Contact,
) []candidate {
	keep := make(map[ids.UUID]bool, len(kept))
	for _, c := range kept {
		keep[c.ID] = true
	}

	out := make([]candidate, 0, len(kept))
	for _, r := range page {
		if !keep[r.Contact.ID] {
			continue
		}

		c := candidate{
			contact: r.Contact.ID, conversation: r.Conversation.ID, status: "queued", due: b.Clock.Now(),
			reason: holdReason(row.Holds, s, r.Conversation),
		}
		if r.Contact.CompanyID != nil {
			c.company = *r.Contact.CompanyID
		}

		if c.reason != "" {
			c.status = "skipped"
		}

		out = append(out, c)
	}

	return out
}

/*
holdReason returns which hold leaves the conversation out, or "" when
it is answered. A customer is waiting when they wrote after the team's
last answer; a teammate already told them when the team answered after
the tracker last moved.
*/
func holdReason(holds []string, s scope, c models.Conversation) string {
	for _, h := range holds {
		switch {
		case h == HoldCustomerWaiting && c.LastCustomerMessageAt != nil &&
			(c.LastReplyAt == nil || c.LastCustomerMessageAt.After(*c.LastReplyAt)):
			return ReasonCustomerWaiting
		case h == HoldTeamReplied && s.since != nil && c.LastReplyAt != nil && c.LastReplyAt.After(*s.since):
			return ReasonTeamReplied
		}
	}

	return ""
}

/*
freezeConversations turns the conversations in scope into recipients.
*/
func (b *Broadcasts) freezeConversations(ctx context.Context, row models.Broadcast) error {
	after := ids.Nil()

	for {
		done := false

		err := b.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
			s, err := scopeOf(ctx, q, row)
			if err != nil {
				return err
			}

			batch, err := b.conversationPage(ctx, q, row, conversationCursor{scope: s, after: after})
			after, done = batch.last, batch.done
			if err != nil {
				return err
			}

			return b.add(ctx, q, row, batch.people)
		})
		if err != nil || done {
			return err
		}
	}
}

/*
estimateConversations counts the people a conversation broadcast would
answer, and those its holds leave out.
*/
func (b *Broadcasts) estimateConversations(
	ctx context.Context, q *models.Queries, row models.Broadcast, out *Estimate,
) error {
	s, err := scopeOf(ctx, q, row)
	if err != nil {
		return err
	}

	after, read := ids.Nil(), 0

	for {
		batch, err := b.conversationPage(ctx, q, row, conversationCursor{scope: s, after: after})
		if err != nil {
			return err
		}

		out.add(batch.people)

		if read += freezePage; batch.done || read >= estimateCap {
			out.Capped = !batch.done

			return nil
		}

		after = batch.last
	}
}

/*
ConversationSender answers each recipient in their conversation, on the
conversation's own channel, as the teammate the broadcast comes from.
*/
type ConversationSender struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Delivery conversations.Delivery
	Now      func() time.Time
}

/*
Ready has nothing to check: whether a conversation's channel can carry
the message is decided per recipient.
*/
func (s *ConversationSender) Ready(context.Context, *models.Queries, models.Broadcast) error {
	return nil
}

/*
Deliver writes the message into the recipient's conversation and hands
it to the conversation's channel. Sending the same recipient twice
writes it once.
*/
func (s *ConversationSender) Deliver(ctx context.Context, d Delivery) (Delivered, error) {
	if d.Recipient.ConversationID == nil {
		return Delivered{}, &UndeliverableError{Status: "failed", Reason: ReasonUnreachable}
	}

	var sent models.Message

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ws := d.Recipient.WorkspaceID

		c, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: ws, ID: *d.Recipient.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return &UndeliverableError{Status: "failed", Reason: ReasonUnreachable}
		}

		if err != nil {
			return wrap("find conversation", err)
		}

		route, err := s.route(ctx, q, c)
		if err != nil {
			return err
		}

		w := &conversations.Writer{Q: q, Tx: tx, Events: s.Events, Now: s.Now(), Actor: s.actor(ctx, q, d.Broadcast)}
		dedupe := "broadcast:" + d.Recipient.ID.String()

		sent, _, err = w.Append(ctx, c, conversations.NewMessage{
			Type: string(enums.MessageTypeMessage), Body: d.Message.Markdown, BodyFormat: new("markdown"),
			DeliveryStatus: &route.Status, DedupeKey: &dedupe, Metadata: broadcastMark(d),
		})
		if err != nil {
			return wrap("answer in conversation", err)
		}

		return s.Delivery.Dispatch(ctx, tx, sent)
	})
	if err != nil {
		return Delivered{}, err
	}

	ref := sent.ID.String()
	pending := sent.DeliveryStatus != nil && *sent.DeliveryStatus == string(enums.DeliveryStatusPending)

	return Delivered{ConversationID: d.Recipient.ConversationID, ExternalRef: &ref, Pending: pending}, nil
}

/*
broadcastMark is what the answer keeps about the broadcast it belongs
to, so the inbox can say so beside it.
*/
func broadcastMark(d Delivery) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"broadcast": map[string]any{
		"id":           models.PublicID(models.PrefixBroadcast, d.Broadcast.ID),
		"recipient_id": models.PublicID(models.PrefixBroadcastRecipient, d.Recipient.ID),
		"name":         d.Broadcast.Name, "channel": d.Broadcast.Channel,
	}})

	return raw
}

/*
ReasonNotDelivered is why a conversation broadcast's answer failed when
its channel gave no reason of its own.
*/
const ReasonNotDelivered = "not_delivered"

/*
MessageDelivery follows what a channel reports about a conversation
broadcast's answer: sent, delivered or read counts the recipient as
delivered, and a final failure fails them with the channel's reason.
Reports about any other message are left alone.
*/
func (b *Broadcasts) MessageDelivery(ctx context.Context, report DeliveryReport) error {
	return b.inWorkspace(withTenant(ctx, report.WorkspaceID), report.WorkspaceID,
		func(ctx context.Context, q *models.Queries) error {
			return b.followDelivery(ctx, q, report)
		})
}

/*
DeliveryReport is what a channel said about one message: its status,
whether no further try follows, and why it failed.
*/
type DeliveryReport struct {
	WorkspaceID ids.UUID
	MessageID   ids.UUID
	Status      string
	Reason      string
	Final       bool
}

/*
followDelivery applies one delivery report to the recipient the
message answered, if any.
*/
func (b *Broadcasts) followDelivery(ctx context.Context, q *models.Queries, report DeliveryReport) error {
	ws, status, reason := report.WorkspaceID, report.Status, report.Reason
	r, err := q.FindBroadcastRecipientByMessage(ctx, models.FindBroadcastRecipientByMessageParams{
		WorkspaceID: ws, ExternalRef: report.MessageID.String(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return wrap("find answered recipient", err)
	}

	switch {
	case status == string(enums.DeliveryStatusFailed) && report.Final:
		if reason == "" {
			reason = ReasonNotDelivered
		}

		_, err = q.MarkBroadcastRecipientFailed(ctx, models.MarkBroadcastRecipientFailedParams{
			WorkspaceID: ws, ID: r.ID, Status: "failed", Reason: &reason, Now: b.Clock.Now(),
		})
	case status == string(enums.DeliveryStatusSent) || status == string(enums.DeliveryStatusDelivered) ||
		status == string(enums.DeliveryStatusRead):
		err = q.MarkBroadcastRecipientDelivered(ctx, models.MarkBroadcastRecipientDeliveredParams{
			WorkspaceID: ws, ID: r.ID, Now: b.Clock.Now(),
		})
	default:
		return nil
	}

	if err := ignoreMissing(err, "follow answer delivery"); err != nil {
		return err
	}

	return b.recount(ctx, ws, r.BroadcastID)
}

/*
route finds how a message reaches the conversation's customer, failing
the recipient for good when nothing can.
*/
func (s *ConversationSender) route(
	ctx context.Context, q *models.Queries, c models.Conversation,
) (conversations.Route, error) {
	conn, err := conversations.Connection(ctx, q, c)
	if err != nil {
		return conversations.Route{}, unreachable(err)
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return conversations.Route{}, unreachable(err)
	}

	route, err := s.Delivery.Plan(conn, contact)

	return route, unreachable(err)
}

/*
unreachable turns a customer or channel that cannot be reached into a
failed recipient, and passes other errors on.
*/
func unreachable(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, exceptions.ErrChannelUnsupported):
		return &UndeliverableError{Status: "failed", Reason: ReasonNoChannel}
	case errors.Is(err, exceptions.ErrContactUnreachable), errors.Is(err, database.ErrNotFound):
		return &UndeliverableError{Status: "failed", Reason: ReasonUnreachable}
	}

	return err
}

/*
actor is who the answer comes from: the teammate the broadcast is sent
as, or dimmi when it names none.
*/
func (s *ConversationSender) actor(ctx context.Context, q *models.Queries, row models.Broadcast) conversations.Actor {
	if row.SenderMemberID == nil {
		return conversations.Actor{Type: string(enums.AuthorTypeSystem)}
	}

	sender, err := q.FindBroadcastSender(ctx, models.FindBroadcastSenderParams{
		WorkspaceID: row.WorkspaceID, ID: *row.SenderMemberID,
	})
	if err != nil {
		return conversations.Actor{Type: string(enums.AuthorTypeSystem)}
	}

	return conversations.Actor{Type: string(enums.AuthorTypeMember), ID: *row.SenderMemberID, Name: sender.Name}
}
