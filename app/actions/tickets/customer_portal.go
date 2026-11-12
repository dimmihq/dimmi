/*
|--------------------------------------------------------------------------
| Requests From the Portal
|--------------------------------------------------------------------------
|
| A signed-in help center reader opens a request with the type's form,
| reads its thread and replies in it. The request's conversation starts
| on the messenger's channel, authored by the reader, so the team
| answers it like any chat and the reader finds it in the messenger
| too. The conversation and its ticket are written together, so a form
| the ticket refuses leaves nothing behind, and a client key a retry
| repeats returns the first request.
|
*/

package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxThread is how many messages a request's page shows.
*/
const maxThread = 200

/*
ErrReplyInvalid refuses an empty or overlong reply.
*/
var ErrReplyInvalid = fault.New(fault.Invalid, "reply_invalid", "Replies are 1 to 10000 characters.").OnField("body")

/*
ErrNoPortalChannel says the workspace has no messenger switched on to
take requests from its help center.
*/
var ErrNoPortalChannel = fault.New(fault.Invalid, "portal_channel_missing",
	"Switch a messenger on to take requests from the help center.")

/*
PortalInput is a reader's form: the ticket's input and the messenger
whose channel its conversation starts on, by public id, the first one
switched on when it names none or one that is off.
*/
type PortalInput struct {
	CustomerInput

	Messenger string
}

/*
PortalCreate starts the reader's conversation and its ticket together.
*/
func (t *Tickets) PortalCreate(ctx context.Context, who Customer, in PortalInput) (Created, error) {
	var out Created

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.contactWriter(ctx, tx, who)
		if err != nil {
			return err
		}

		if found, ok, err := repeated(ctx, w.Q, who.WorkspaceID, in.ExternalID); ok || err != nil {
			out = Created{Ticket: found}

			return err
		}

		if err := checkCustomerForm(w.Setup, in.CustomerInput); err != nil {
			return err
		}

		c, err := w.startConversation(ctx, who, in)
		if err != nil {
			return err
		}

		in.ConversationID = c.ID
		out, err = w.Create(ctx, who.WorkspaceID, CreateInput{
			TypeID: in.TypeID, Title: in.Title, Description: in.Description, Fields: in.Fields, Customer: true,
			ConversationID: &c.ID, ContactID: &who.ContactID, ExternalID: in.ExternalID,
		})

		return err
	})

	return out, err
}

/*
repeated finds the ticket a retried form made the first time.
*/
func repeated(ctx context.Context, q *models.Queries, ws ids.UUID, key *string) (models.Ticket, bool, error) {
	if key == nil {
		return models.Ticket{}, false, nil
	}

	found, err := q.FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{WorkspaceID: ws, ExternalID: key})
	if errors.Is(err, database.ErrNotFound) {
		return found, false, nil
	}

	if err != nil {
		return found, false, fmt.Errorf("find repeated request: %w", err)
	}

	return found, true, nil
}

/*
startConversation opens the request's conversation on the channel,
with the reader's words as its first message.
*/
func (w *Writer) startConversation(ctx context.Context, who Customer, in PortalInput) (models.Conversation, error) {
	messenger, err := portalMessenger(ctx, w.Q, who.WorkspaceID, in.Messenger)
	if err != nil {
		return models.Conversation{}, err
	}

	conn, err := w.Q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: who.WorkspaceID, ID: messenger.ChannelConnectionID,
	})
	if err != nil || conn.DeletedAt != nil {
		return models.Conversation{}, ErrNoPortalChannel
	}

	number, err := w.Q.AllocateConversationNumber(ctx, who.WorkspaceID)
	if err != nil {
		return models.Conversation{}, fmt.Errorf("allocate conversation number: %w", err)
	}

	source, _ := json.Marshal(map[string]string{
		"widget_id": models.PublicID(models.PrefixWidget, messenger.ID), "via": "help_center",
	})

	subject := strings.TrimSpace(in.Title)

	c, err := w.Q.CreateConversation(ctx, models.CreateConversationParams{
		WorkspaceID: who.WorkspaceID, Number: number, InboxID: conn.InboxID, ChannelConnectionID: conn.ID,
		ContactID: who.ContactID, Subject: &subject, Status: string(enums.ConversationStatusOpen), Source: source,
		Now: w.Now,
	})
	if err != nil {
		return c, fmt.Errorf("start request conversation: %w", err)
	}

	if err := w.Dispatch(ctx, appevents.ConversationCreated{
		Base: w.Base(c.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: c.ID},
		ContactID: c.ContactID, InitiatedBy: string(enums.AuthorTypeContact),
	}); err != nil {
		return c, err
	}

	body := strings.TrimSpace(in.Description)
	if body == "" {
		body = subject
	}

	_, c, err = w.Append(ctx, c, conversations.NewMessage{Type: string(enums.MessageTypeMessage), Body: body})

	return c, err
}

/*
portalMessenger finds the messenger a portal request starts on: the
one named when it is on, else the first one on.
*/
func portalMessenger(ctx context.Context, q *models.Queries, ws ids.UUID, named string) (models.WidgetSetting, error) {
	all, err := q.ListWidgetSettings(ctx, ws)
	if err != nil {
		return models.WidgetSetting{}, fmt.Errorf("list messengers: %w", err)
	}

	var first *models.WidgetSetting

	for i, m := range all {
		if !m.Enabled {
			continue
		}

		if models.PublicID(models.PrefixWidget, m.ID) == named {
			return m, nil
		}

		if first == nil {
			first = &all[i]
		}
	}

	if first == nil {
		return models.WidgetSetting{}, ErrNoPortalChannel
	}

	return *first, nil
}

/*
PortalThread is a request with the messages its customer sees, oldest
first.
*/
type PortalThread struct {
	CustomerTicket

	Messages []models.Message
}

/*
PortalShow returns one of the reader's requests with its thread.
*/
func (t *Tickets) PortalShow(ctx context.Context, who Customer, ref Ref) (PortalThread, error) {
	shown, err := t.CustomerShow(ctx, who, ref)
	if err != nil || shown.Ticket.ConversationID == nil {
		return PortalThread{CustomerTicket: shown}, err
	}

	out := PortalThread{CustomerTicket: shown}

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	err = t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListWidgetMessages(ctx, models.ListWidgetMessagesParams{
			WorkspaceID: who.WorkspaceID, ConversationID: *shown.Ticket.ConversationID, RowLimit: maxThread,
		})
		if err != nil {
			return fmt.Errorf("list request messages: %w", err)
		}

		for _, row := range slices.Backward(rows) {
			if row.DeletedAt == nil {
				out.Messages = append(out.Messages, row)
			}
		}

		return nil
	})

	return out, err
}

/*
PortalReply writes the reader's reply in their request's conversation.
*/
func (t *Tickets) PortalReply(ctx context.Context, who Customer, ref Ref, body string) error {
	body = strings.TrimSpace(body)
	if n := len([]rune(body)); n < 1 || n > maxLongText {
		return ErrReplyInvalid
	}

	ctx = tenancy.With(ctx, who.WorkspaceID.String())

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.contactWriter(ctx, tx, who)
		if err != nil {
			return err
		}

		r, err := findRow(ctx, w.Q, who.WorkspaceID, ref)
		if err != nil {
			return err
		}

		if !customerSees(w.Setup, who, r) || r.ConversationID == nil {
			return ErrTicketNotFound
		}

		c, err := w.Q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: who.WorkspaceID, ID: *r.ConversationID,
		})
		if err != nil {
			return fmt.Errorf("find request conversation: %w", err)
		}

		_, _, err = w.Append(ctx, c, conversations.NewMessage{Type: string(enums.MessageTypeMessage), Body: body})

		return err
	})
}
