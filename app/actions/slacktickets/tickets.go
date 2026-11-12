/*
|--------------------------------------------------------------------------
| Making The Ticket
|--------------------------------------------------------------------------
|
| A marked thread's conversation becomes a customer ticket through the
| same writer every other path uses, acting as the teammate who marked
| it when they are linked to a member, else as the Slack integration.
| Its type is the room's ticket_type_id setting, else the type the
| conversation's inbox turns conversations into, else the workspace's
| default_type_id ticket setting, else the first customer type. Its
| title is the conversation's subject, else the thread's first line.
|
*/

package slacktickets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
titleLimit is the longest title a ticket takes.
*/
const titleLimit = 200

/*
makeTicket turns the marked thread's conversation into a ticket, or
returns the ticket it already is.
*/
func (i *Intake) makeTicket(
	ctx context.Context, conn models.ChannelConnection, marked channels.MarkedThread,
) (tickets.Created, error) {
	var out tickets.Created

	err := i.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		conv, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: conn.WorkspaceID, ID: marked.ConversationID,
		})
		if err != nil {
			return fmt.Errorf("find marked conversation: %w", err)
		}

		found, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{
			WorkspaceID: conn.WorkspaceID, ConversationID: &conv.ID,
		})
		if err == nil {
			out = tickets.Created{Ticket: found}

			return nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find conversation ticket: %w", err)
		}

		out, err = i.create(ctx, tx, conv, marked)

		return err
	})

	return out, err
}

/*
create makes the conversation a ticket within the transaction.
*/
func (i *Intake) create(
	ctx context.Context, tx *database.Tx, conv models.Conversation, marked channels.MarkedThread,
) (tickets.Created, error) {
	cw := &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: i.Events, Now: i.Clock.Now(), Actor: actorOf(marked),
	}

	w, err := i.Tickets.Wrap(ctx, cw, conv.WorkspaceID)
	if err != nil {
		return tickets.Created{}, err
	}

	typeID, err := i.typeOf(ctx, w, conv, marked.Room)
	if err != nil {
		return tickets.Created{}, err
	}

	return w.Create(ctx, conv.WorkspaceID, tickets.CreateInput{
		TypeID: typeID, ConversationID: &conv.ID, Title: titleOf(conv, marked),
	})
}

/*
actorOf is who makes the ticket: the linked teammate, else the Slack
integration.
*/
func actorOf(marked channels.MarkedThread) conversations.Actor {
	if marked.MemberID == nil {
		return conversations.Actor{Type: string(enums.AuthorTypeIntegration), Name: "Slack"}
	}

	return conversations.Actor{
		Type: string(enums.AuthorTypeMember), ID: *marked.MemberID, Name: marked.MemberName,
	}
}

/*
typeOf picks the ticket's type: the room's, the inbox's, the
workspace's default, then the first customer type.
*/
func (i *Intake) typeOf(
	ctx context.Context, w *tickets.Writer, conv models.Conversation, room models.ChannelRoom,
) (ids.UUID, error) {
	candidates := []string{channels.RoomSetting(room, "ticket_type_id")}

	inboxType, err := w.Q.FindInboxTicketType(ctx, models.FindInboxTicketTypeParams{
		WorkspaceID: conv.WorkspaceID, ID: conv.InboxID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return ids.Nil(), fmt.Errorf("find inbox ticket type: %w", err)
	}

	if inboxType != nil {
		candidates = append(candidates, inboxType.String())
	}

	settings, err := w.Q.FindWorkspaceTicketSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return ids.Nil(), fmt.Errorf("find ticket settings: %w", err)
	}

	candidates = append(candidates, defaultType(settings.TicketSettings))

	for _, c := range candidates {
		if id, ok := i.customerType(w.Setup, c); ok {
			return id, nil
		}
	}

	ty, ok := w.Setup.FirstType(tickets.KindCustomer)
	if !ok {
		return ids.Nil(), tickets.ErrTypeNotFound
	}

	return ty.ID, nil
}

/*
defaultType reads the workspace's default type from its ticket
settings, empty when it names none.
*/
func defaultType(raw json.RawMessage) string {
	var settings map[string]any
	if json.Unmarshal(raw, &settings) != nil {
		return ""
	}

	id, _ := settings["default_type_id"].(string)

	return id
}

/*
customerType reads a type id, plain or public, and reports whether it
names a live customer type.
*/
func (i *Intake) customerType(setup tickets.Setup, raw string) (ids.UUID, bool) {
	if raw == "" {
		return ids.Nil(), false
	}

	id, err := ids.Parse(raw)
	if err != nil {
		if id, err = i.IDs.Decode(string(models.PrefixTicketType), raw); err != nil {
			return ids.Nil(), false
		}
	}

	ty, ok := setup.Type(id)
	if !ok || ty.Kind != tickets.KindCustomer || ty.ArchivedAt != nil {
		return ids.Nil(), false
	}

	return id, true
}

/*
titleOf is the ticket's title: the subject, the thread's first line,
or the conversation's number.
*/
func titleOf(conv models.Conversation, marked channels.MarkedThread) string {
	if conv.Subject != nil && strings.TrimSpace(*conv.Subject) != "" {
		return cut(*conv.Subject)
	}

	line, _, _ := strings.Cut(strings.TrimSpace(marked.Root.Text), "\n")
	if line = strings.TrimSpace(line); line != "" {
		return cut(line)
	}

	return "Conversation #" + strconv.FormatInt(conv.Number, 10)
}

/*
cut keeps a title within the limit.
*/
func cut(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > titleLimit {
		return strings.TrimSpace(string(r[:titleLimit]))
	}

	return s
}
