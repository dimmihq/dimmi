/*
|--------------------------------------------------------------------------
| Slack Ticket Intake
|--------------------------------------------------------------------------
|
| In a connected Slack channel a teammate turns a customer's thread into
| a ticket two ways: the ticket emoji on any message of the thread, or
| the "Make a dimmi ticket" message shortcut. The thread's conversation
| is found, or opened when nothing opened it yet, and becomes a ticket of
| the room's type, else the inbox's, else the workspace's default. A
| thread that is a ticket already stays one ticket. Customers' reactions
| are left alone. State changes then reach the thread the way every
| reply does, through the conversation's own channel.
|
*/

package slacktickets

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Emoji is the reaction that turns a thread into a ticket.
*/
const Emoji = "ticket"

/*
Intake turns Slack threads into tickets. AppURL is the install's
address, for the link a shortcut answers with.
*/
type Intake struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	IDs     *ids.Registry
	Slack   *channels.Slack
	Tickets *tickets.Tickets
	AppURL  string
}

/*
Claim takes the ticket emoji ahead of the channel's own reaction
handlers, and the ticket shortcut.
*/
func (i *Intake) Claim(r *channels.Router) {
	r.OnEventFirst("reaction_added", i.onReaction)
	r.OnShortcut(slackdriver.ShortcutTicket, i.onShortcut)
}

/*
onReaction makes a ticket of the thread a teammate marked with the
ticket emoji, leaving every other reaction to other features.
*/
func (i *Intake) onReaction(ctx context.Context, ev channels.SlackEvent) (bool, error) {
	item := ev.Event.Item
	if !ev.Connected() || ev.Event.Reaction != Emoji || item.Type != "message" || item.Channel == "" {
		return false, nil
	}

	ctx = tenancy.With(ctx, ev.WorkspaceID.String())

	marked, err := i.Slack.MarkThread(ctx, channels.ThreadMark{
		Conn: ev.Connection, Channel: item.Channel, TS: item.TS, User: ev.Event.User,
		Trigger: enums.RoomTriggerEmoji,
	})
	if refused(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	_, err = i.makeTicket(ctx, ev.Connection, marked)

	return err == nil, err
}

/*
onShortcut makes a ticket of the thread the shortcut was used on and
tells the teammate its reference, or why not.
*/
func (i *Intake) onShortcut(ctx context.Context, in channels.SlackInteraction) (bool, error) {
	msg := in.Interaction.Message
	if !in.Connected() || msg == nil {
		return false, nil
	}

	ctx = tenancy.With(ctx, in.WorkspaceID.String())

	marked, err := i.Slack.MarkThread(ctx, channels.ThreadMark{
		Conn: in.Connection, Channel: in.Interaction.ChannelID(), TS: msg.TS, User: in.Interaction.User.ID,
		Message: msg, Trigger: enums.RoomTriggerShortcut,
	})
	if refused(err) {
		i.Slack.Whisper(ctx, in, sentence(err))

		return true, nil
	}

	if err != nil {
		return false, err
	}

	made, err := i.makeTicket(ctx, in.Connection, marked)
	if err != nil {
		return false, err
	}

	verb := "Made ticket "
	if !made.New {
		verb = "Already ticket "
	}

	i.Slack.Whisper(ctx, in, verb+tickets.Reference(made.Ticket.Number)+": "+i.link(marked.ConversationID))

	return true, nil
}

/*
refused reports whether the mark was refused for a reason the teammate
can read, rather than failing.
*/
func refused(err error) bool {
	return errors.Is(err, channels.ErrMarkUnwatched) || errors.Is(err, channels.ErrMarkCustomer) ||
		errors.Is(err, channels.ErrMarkNoThread)
}

/*
sentence writes a refusal as a sentence for the teammate.
*/
func sentence(err error) string {
	text := err.Error()

	return strings.ToUpper(text[:1]) + text[1:] + "."
}

/*
link is the inbox address of the ticket's conversation.
*/
func (i *Intake) link(conversationID ids.UUID) string {
	return i.AppURL + "/d/inbox/all/" + models.PublicID(models.PrefixConversation, conversationID)
}
