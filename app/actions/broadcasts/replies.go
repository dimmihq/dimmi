/*
|--------------------------------------------------------------------------
| Replies Become Conversations
|--------------------------------------------------------------------------
|
| However a person answers a broadcast, by email, in the Messenger or in
| the Slack thread of the post, the answer lands in one conversation
| that starts with the broadcast, goes to the team or teammate the
| broadcast chose, and counts as a reply in its report. A second answer
| from the same person joins the same conversation.
|
*/

package broadcasts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailReply returns the conversation of the broadcast email the tokens
or Message-IDs name, opening it on the first answer.
*/
func (b *Broadcasts) EmailReply(
	ctx context.Context, tx *database.Tx, ws ids.UUID, refs email.ReplyRefs,
) (*models.Conversation, error) {
	q := models.New(tx)

	r, found, err := emailRecipient(ctx, q, ws, refs)
	if err != nil || !found {
		return nil, err
	}

	if c, ok, err := existing(ctx, q, r); err != nil || ok {
		return c, err
	}

	row, err := find(ctx, q, ws, r.BroadcastID)
	if err != nil || row.EmailAddressID == nil || r.ContactID == nil {
		return nil, gone(err)
	}

	conn, err := emailConnection(ctx, q, ws, *row.EmailAddressID)
	if err != nil {
		return nil, err
	}

	o, err := b.opening(ctx, q, row, r)
	if err != nil {
		return nil, err
	}

	o.conn = conn
	o.thread = r.MessageIDHeader

	c, err := b.started(ctx, tx, o)
	if err != nil {
		return nil, err
	}

	return &c, b.fileMessageID(ctx, q, r, c)
}

/*
SlackOpened writes the broadcast post a Slack thread answers into the
conversation the first answer starts, routes it and counts the reply.
A thread no broadcast started is left alone.
*/
func (b *Broadcasts) SlackOpened(
	ctx context.Context, tx *database.Tx, c models.Conversation, t channels.SlackThread,
) error {
	q := models.New(tx)
	room, rootTS := t.Room, t.RootTS

	r, err := q.FindBroadcastRecipientByRef(ctx, models.FindBroadcastRecipientByRefParams{
		WorkspaceID: room.WorkspaceID, RoomID: &room.ID, ExternalRef: &rootTS,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return wrap("find broadcast post", err)
	}

	row, err := find(ctx, q, room.WorkspaceID, r.BroadcastID)
	if err != nil {
		return gone(err)
	}

	o, err := b.opening(ctx, q, row, r)
	if err != nil {
		return err
	}

	o.body = slackBody(Delivery{Message: Rendered{Subject: o.subjectText(), Markdown: o.body}})

	w, err := b.writerFor(ctx, tx, row)
	if err != nil {
		return err
	}

	sentAt(w, r)

	if _, c, err = firstMessage(ctx, w, c, o); err != nil {
		return err
	}

	if err := b.route(ctx, tx, row, c); err != nil {
		return err
	}

	return b.replied(ctx, tx, r, c.ID)
}

/*
writerFor is a conversation writer acting as the broadcast's sender.
*/
func (b *Broadcasts) writerFor(
	ctx context.Context, tx *database.Tx, row models.Broadcast) (*conversations.Writer, error,
) {
	if row.SenderMemberID == nil {
		return nil, ErrNoSender
	}

	m, ok, err := slackteam.Membership(ctx, models.New(tx), row.WorkspaceID, *row.SenderMemberID)
	if err != nil || !ok {
		return nil, orSender(err)
	}

	return conversations.NewWriter(ctx, tx, conversations.Services{Events: b.Events, Clock: b.Clock}, m)
}

/*
MessengerReplied counts a contact's answer in a Messenger conversation
a broadcast started and hands it to the broadcast's team.
*/
func (b *Broadcasts) MessengerReplied(ctx context.Context, tx *database.Tx, c models.Conversation) error {
	q := models.New(tx)

	r, err := q.FindBroadcastRecipientByConversation(ctx, models.FindBroadcastRecipientByConversationParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return wrap("find broadcast recipient", err)
	}

	if r.RepliedAt != nil {
		return nil
	}

	row, err := find(ctx, q, c.WorkspaceID, r.BroadcastID)
	if err != nil {
		return gone(err)
	}

	if err := b.route(ctx, tx, row, c); err != nil {
		return err
	}

	return b.replied(ctx, tx, r, c.ID)
}

/*
started opens the conversation of an answer, routes it and counts the
reply.
*/
func (b *Broadcasts) started(ctx context.Context, tx *database.Tx, o opening) (models.Conversation, error) {
	o.status = string(enums.ConversationStatusOpen)

	out, err := open(ctx, tx, conversations.Services{Events: b.Events, Clock: b.Clock}, o)
	if err != nil {
		return models.Conversation{}, err
	}

	c := out.conversation
	if err := b.route(ctx, tx, o.row, c); err != nil {
		return c, err
	}

	return c, b.replied(ctx, tx, o.r, c.ID)
}

/*
route hands an answered conversation to the teammate or team the
broadcast chose, unless someone already holds it.
*/
func (b *Broadcasts) route(ctx context.Context, tx *database.Tx, row models.Broadcast, c models.Conversation) error {
	if (row.ReplyMemberID == nil && row.ReplyTeamID == nil) || c.AssigneeMemberID != nil || c.AssigneeTeamID != nil {
		return nil
	}

	w := &conversations.Writer{Q: models.New(tx), Tx: tx, Events: b.Events, Now: b.Clock.Now()}
	w = w.AsSystem()

	_, _, err := w.Assign(ctx, c, conversations.Assignment{Member: row.ReplyMemberID, Team: row.ReplyTeamID})

	return err
}

/*
replied counts the answer on the recipient and tells the broadcast.
*/
func (b *Broadcasts) replied(ctx context.Context, tx *database.Tx, r models.BroadcastRecipient, c ids.UUID) error {
	if _, err := models.New(tx).MarkBroadcastRecipientReplied(ctx, models.MarkBroadcastRecipientRepliedParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, ConversationID: &c, Now: b.Clock.Now(),
	}); err != nil {
		return wrap("mark replied", err)
	}

	return b.Events.Dispatch(ctx, tx, appevents.BroadcastRecipientChanged{
		Base:        appevents.Base{ID: ids.New(), WorkspaceID: r.WorkspaceID, OccurredAt: b.Clock.Now()},
		BroadcastID: r.BroadcastID, RecipientID: r.ID,
	})
}

/*
opening is the conversation a recipient's answer opens, with the
broadcast as the recipient got it; the caller sets the channel it
lives on.
*/
func (b *Broadcasts) opening(
	ctx context.Context, q *models.Queries, row models.Broadcast, r models.BroadcastRecipient,
) (opening, error) {
	d := Delivery{Broadcast: row, Recipient: r}
	if err := b.subject(ctx, q, &d); err != nil {
		return opening{}, err
	}

	message, err := b.render(ctx, q, d)
	if err != nil {
		return opening{}, err
	}

	o := opening{row: row, r: r, body: message.Markdown, subject: optionalText(message.Subject)}
	if d.Contact != nil {
		o.contact = *d.Contact
	}

	return o, nil
}

/*
subjectText is the opening's subject, empty when it has none.
*/
func (o opening) subjectText() string {
	if o.subject == nil {
		return ""
	}

	return *o.subject
}

/*
existing returns the conversation a recipient's earlier answer opened.
*/
func existing(ctx context.Context, q *models.Queries, r models.BroadcastRecipient) (*models.Conversation, bool, error) {
	if r.ConversationID == nil {
		return nil, false, nil
	}

	c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: r.WorkspaceID, ID: *r.ConversationID})
	if errors.Is(err, database.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, wrap("find conversation", err)
	}

	return &c, true, nil
}

/*
emailRecipient finds the recipient a reply names, by token first.
*/
func emailRecipient(
	ctx context.Context, q *models.Queries, ws ids.UUID, refs email.ReplyRefs,
) (models.BroadcastRecipient, bool, error) {
	messageIDs := refs.MessageIDs
	for _, token := range refs.Tokens {
		r, err := q.FindBroadcastRecipientByReplyToken(ctx, models.FindBroadcastRecipientByReplyTokenParams{
			WorkspaceID: ws, ReplyToken: token,
		})
		if err == nil {
			return r, true, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return r, false, wrap("find broadcast reply token", err)
		}
	}

	if len(messageIDs) == 0 {
		return models.BroadcastRecipient{}, false, nil
	}

	r, err := q.FindBroadcastRecipientByMessageID(ctx, models.FindBroadcastRecipientByMessageIDParams{
		WorkspaceID: ws, MessageIds: messageIDs,
	})
	if errors.Is(err, database.ErrNotFound) {
		return r, false, nil
	}

	return r, err == nil, wrap("find broadcast message id", err)
}

/*
emailConnection finds the channel of the address a broadcast was sent
from.
*/
func emailConnection(ctx context.Context, q *models.Queries, ws, addressID ids.UUID) (models.ChannelConnection, error) {
	address, err := q.FindEmailAddressByID(ctx, models.FindEmailAddressByIDParams{WorkspaceID: ws, ID: addressID})
	if err != nil {
		return models.ChannelConnection{}, wrap("find email address", err)
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: ws, ID: address.ChannelConnectionID,
	})

	return conn, wrap("find email channel", err)
}

/*
fileMessageID files the broadcast's Message-ID under the conversation,
so later answers that only quote it thread there too.
*/
func (b *Broadcasts) fileMessageID(
	ctx context.Context, q *models.Queries, r models.BroadcastRecipient, c models.Conversation,
) error {
	if r.MessageIDHeader == nil {
		return nil
	}

	return wrap("file broadcast message id", q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
		WorkspaceID: r.WorkspaceID, RfcMessageID: *r.MessageIDHeader, ConversationID: c.ID,
		Kind: string(enums.EmailRefKindOutbound), Now: b.Clock.Now(),
	}))
}

/*
SlackReacted counts a reaction on a broadcast post toward its report.
*/
func (b *Broadcasts) SlackReacted(ctx context.Context, post channels.SlackThread, delta int) error {
	ws := post.Room.WorkspaceID

	return b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		r, err := q.FindBroadcastRecipientByRef(ctx, models.FindBroadcastRecipientByRefParams{
			WorkspaceID: ws, RoomID: &post.Room.ID, ExternalRef: &post.RootTS,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return wrap("find broadcast post", err)
		}

		if err := q.CountBroadcastRecipientReaction(ctx, models.CountBroadcastRecipientReactionParams{
			WorkspaceID: ws, ID: r.ID, Delta: int32(max(min(delta, 1), -1)), Now: b.Clock.Now(),
		}); err != nil {
			return wrap("count reaction", err)
		}

		return b.recount(ctx, ws, r.BroadcastID)
	})
}

/*
sentAt dates what the writer writes to when the broadcast reached the
recipient, so the broadcast sits above the answer that came after it.
*/
func sentAt(w *conversations.Writer, r models.BroadcastRecipient) {
	if r.SentAt != nil && r.SentAt.Before(w.Now) {
		w.Now = *r.SentAt
	}
}
