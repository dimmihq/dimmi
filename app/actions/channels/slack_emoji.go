/*
|--------------------------------------------------------------------------
| Slack Emoji Actions
|--------------------------------------------------------------------------
|
| A teammate working a customer room acts with a reaction on the
| customer's first message: a check mark closes the conversation, eyes
| take it and open it, a siren makes it urgent; taking the check mark
| off opens it again. Only linked teammates act this way, so a customer
| reacting to their own message changes nothing, and the app's own
| marks never act on what they mark. A room may switch emoji actions
| off, or name other emoji for each.
|
*/

package channels

import (
	"cmp"
	"context"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The default emoji of each action, and the room settings that rename
them.
*/
const (
	emojiClose        = "white_check_mark"
	emojiTake         = "eyes"
	emojiUrgent       = "rotating_light"
	settingEmojiClose = "emoji_close"
	settingEmojiTake  = "emoji_take"
	settingEmojiUrge  = "emoji_urgent"
)

/*
emojiAct is a reaction on its way to an action: who reacted, on which
conversation, and with what.
*/
type emojiAct struct {
	who      teammate
	conv     models.Conversation
	room     models.ChannelRoom
	reaction string
	added    bool
}

/*
onEmojiAction acts on a teammate's reaction to a customer's message.
*/
func (s *Slack) onEmojiAction(ctx context.Context, ev SlackEvent) (bool, error) {
	act, ok, err := s.emojiActOf(ctx, ev)
	if err != nil || !ok {
		return false, err
	}

	return s.actOnEmoji(ctx, act)
}

/*
emojiActOf finds who reacted and on which conversation, reporting
false for a reaction that never acts.
*/
func (s *Slack) emojiActOf(ctx context.Context, ev SlackEvent) (emojiAct, bool, error) {
	item := ev.Event.Item
	if item.Type != "message" || ev.Event.User == "" || ev.Event.User == FactsOf(ev.Connection).BotUserID {
		return emojiAct{}, false, nil
	}

	room, ok, err := s.watchedRoom(ctx, ev.Connection, item.Channel)
	if err != nil || !ok || !customerRoom(&room) || !enabledOf(&room, settingEmojiActions, true) {
		return emojiAct{}, false, err
	}

	conv, found, err := s.threadConversation(ctx, ev.Connection, slackdriver.RoomThreadKey(room.ID, item.TS))
	if err != nil || !found {
		return emojiAct{}, false, err
	}

	a, err := s.classify(ctx, ev.Connection, &room, ev.Event.User)
	if err != nil || a.memberID == nil {
		return emojiAct{}, false, err
	}

	return emojiAct{
		who: teammate{memberID: *a.memberID, name: a.memberName, userID: ev.Event.User}, conv: conv, room: room,
		reaction: ev.Event.Reaction, added: ev.Event.Type == "reaction_added",
	}, true, nil
}

/*
actOnEmoji does what the reaction stands for.
*/
func (s *Slack) actOnEmoji(ctx context.Context, act emojiAct) (bool, error) {
	closeWith := cmp.Or(settingOf(&act.room, settingEmojiClose), emojiClose)

	switch {
	case act.reaction == closeWith:
		status := string(enums.ConversationStatusClosed)
		if !act.added {
			status = string(enums.ConversationStatusOpen)
		}

		return true, s.moveTo(ctx, act.who, act.conv, status)
	case !act.added:
		return false, nil
	case act.reaction == cmp.Or(settingOf(&act.room, settingEmojiTake), emojiTake):
		return true, s.takeAndOpen(ctx, act)
	case act.reaction == cmp.Or(settingOf(&act.room, settingEmojiUrge), emojiUrgent):
		return true, s.markUrgent(ctx, act.who, act.conv)
	default:
		return false, nil
	}
}

/*
takeAndOpen assigns the conversation to the teammate who reacted and
opens it.
*/
func (s *Slack) takeAndOpen(ctx context.Context, act emojiAct) error {
	return s.asTeammate(ctx, act.who, func(ctx context.Context, w *conversations.Writer) error {
		conv, _, err := w.Assign(ctx, act.conv, conversations.Assignment{
			Member: &act.who.memberID, Team: act.conv.AssigneeTeamID,
		})
		if err != nil {
			return err
		}

		_, _, err = w.ChangeStatus(ctx, conv, conversations.StatusChange{
			Status: string(enums.ConversationStatusOpen), Reason: conversations.ReasonManual,
		})

		return ignoreRefusal(err)
	})
}
