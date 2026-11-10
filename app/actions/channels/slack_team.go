/*
|--------------------------------------------------------------------------
| Slack Team Work
|--------------------------------------------------------------------------
|
| Beside the customer's thread the team works the conversation in its
| own channels: a triage card, discussions, the agent's drafts. Each of
| these follows the inbox the same way the customer's thread does: an
| inbox change queues a job after it committed, and the job brings
| Slack up to date. A teammate acting from Slack acts as their linked
| member, so dimmi records who did what; someone not linked is asked to
| link their account first and changes nothing.
|
| Room settings are read as they are stored, numbers and switches with
| their defaults, so a room nobody configured behaves as it always did.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Room setting keys the team's work reads, their limits and defaults, the
notice for a Slack user who is not linked, how long card updates wait
to gather a burst, and how many drafts the agreement rate counts.
*/
const (
	settingAgentMode      = "agent_mode"
	settingAgentDelay     = "agent_delay_seconds"
	settingAgentDelayWhen = "agent_delay_when"
	settingTriageChannel  = "triage_channel_id"
	settingAutoReplyAfter = "auto_reply_after_seconds"
	settingAutoReplyText  = "auto_reply_message"
	settingAutoReplyWhen  = "auto_reply_when"
	settingAutoJoin       = "auto_join"
	settingEmojiActions   = "emoji_actions"
	maxDelaySeconds       = 900
	defaultAutoReplyText  = "Thanks for reaching out. Someone from the team will reply here soon."
	agentModeDraft        = "draft"
	whenInsideHours       = "inside_hours"
	whenOutsideHours      = "outside_hours"
	linkFirstNotice       = "Link your Slack account in dimmi first, so what you do here is recorded as you."
	effectDebounce        = 2
	shadowWindow          = 50
)

/*
numberOf returns one of a room's number settings, zero when unset or
not a number.
*/
func numberOf(room *models.ChannelRoom, key string) int {
	raw, ok := rawSetting(room, key)
	if !ok {
		return 0
	}

	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int(n)
	}

	n, _ = strconv.ParseFloat(settingOf(room, key), 64)

	return int(n)
}

/*
enabledOf returns one of a room's switches, the fallback when unset.
*/
func enabledOf(room *models.ChannelRoom, key string, fallback bool) bool {
	raw, ok := rawSetting(room, key)
	if !ok {
		return fallback
	}

	var on bool
	if json.Unmarshal(raw, &on) != nil {
		return fallback
	}

	return on
}

/*
rawSetting returns a room setting as stored, reporting false when unset.
*/
func rawSetting(room *models.ChannelRoom, key string) (json.RawMessage, bool) {
	if room == nil {
		return nil, false
	}

	var settings map[string]json.RawMessage
	if json.Unmarshal(room.Settings, &settings) != nil {
		return nil, false
	}

	raw, ok := settings[key]

	return raw, ok && string(raw) != "null"
}

/*
connectionSetting reads one key of the connection's config into out,
reporting false when it is unset or unreadable.
*/
func connectionSetting(conn models.ChannelConnection, key string, out any) bool {
	var config map[string]json.RawMessage
	if json.Unmarshal(conn.Config, &config) != nil {
		return false
	}

	raw, ok := config[key]

	return ok && json.Unmarshal(raw, out) == nil
}

/*
customerRoom reports whether customers write in the room, so its
conversations get triage, auto-replies and emoji actions.
*/
func customerRoom(room *models.ChannelRoom) bool {
	return room != nil && room.Mode != string(enums.RoomModeInternal) &&
		room.Status == string(enums.RoomStatusActive)
}

/*
roomOfConversation returns the room a conversation lives in, reporting
false for a direct message or a room gone.
*/
func (s *Slack) roomOfConversation(ctx context.Context, conv models.Conversation) (models.ChannelRoom, bool, error) {
	thread, ok := threadOf(conv)
	if !ok || thread.Direct() {
		return models.ChannelRoom{}, false, nil
	}

	var room models.ChannelRoom

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		room, err = q.FindChannelRoom(ctx, models.FindChannelRoomParams{
			WorkspaceID: conv.WorkspaceID, ID: thread.RoomID,
		})

		return err
	})
	if err != nil {
		return room, false, ignoreMissing(err)
	}

	return room, true, nil
}

/*
findConversation returns the conversation with the id.
*/
func (s *Slack) findConversation(ctx context.Context, workspaceID, id ids.UUID) (models.Conversation, error) {
	var conv models.Conversation

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conv, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: id})

		return err
	})

	return conv, err
}

/*
conversationByPublicID finds the conversation a button names, on the
connection, reporting false for any other.
*/
func (s *Slack) conversationByPublicID(ctx context.Context, conn models.ChannelConnection, publicID string) (
	models.Conversation, bool, error,
) {
	id, err := s.IDs.Decode(string(models.PrefixConversation), publicID)
	if err != nil {
		return models.Conversation{}, false, ignoreUnreadable(err)
	}

	conv, err := s.findConversation(ctx, conn.WorkspaceID, id)
	if err != nil {
		return conv, false, ignoreMissing(err)
	}

	return conv, conv.ChannelConnectionID == conn.ID && conv.DeletedAt == nil, nil
}

/*
teammate is a Slack user acting for the team, as their linked member.
*/
type teammate struct {
	memberID ids.UUID
	name     string
	userID   string
}

/*
teammateOf returns the linked member the Slack user acts as, telling
them to link their account and reporting false when there is none.
*/
func (s *Slack) teammateOf(ctx context.Context, in SlackInteraction) (teammate, bool, error) {
	a, err := s.classify(ctx, in.Connection, nil, in.Interaction.User.ID)
	if err != nil {
		return teammate{}, false, err
	}

	if a.memberID == nil {
		s.tell(ctx, in, linkFirstNotice)

		return teammate{}, false, nil
	}

	return teammate{memberID: *a.memberID, name: a.memberName, userID: in.Interaction.User.ID}, true, nil
}

/*
asTeammate runs the change as the teammate, in one transaction with a
writer acting as them.
*/
func (s *Slack) asTeammate(ctx context.Context, who teammate,
	fn func(ctx context.Context, w *conversations.Writer) error,
) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &conversations.Writer{Q: models.New(tx), Tx: tx, Events: s.Events, Now: s.Clock.Now()}

		return fn(ctx, w.AsMember(who.memberID, who.name))
	})
}

/*
teamReply is a reply a teammate sends the customer from Slack: the
conversation, the text and what the message keeps of where it came
from.
*/
type teamReply struct {
	conv models.Conversation
	body string
	meta json.RawMessage
}

/*
replyAs posts a reply to the customer as the teammate: stored as their
reply and sent through the normal outbound, with their own Slack
account when they connected it.
*/
func (s *Slack) replyAs(ctx context.Context, who teammate, r teamReply) error {
	return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		msg, _, err := w.Append(ctx, r.conv, conversations.NewMessage{
			Type: string(enums.MessageTypeMessage), Body: r.body, BodyFormat: new(string(enums.BodyFormatMarkdown)),
			DeliveryStatus: new(string(enums.DeliveryStatusPending)), Metadata: r.meta,
		})
		if err != nil {
			return fmt.Errorf("reply from slack: %w", err)
		}

		return s.Queue.PushTx(ctx, w.Tx, jobs.SendChannelMessageArgs{
			WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID,
		})
	})
}

/*
applyTeam applies a change on the team's side of Slack.
*/
func (s *Slack) applyTeam(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	args jobs.ApplySlackEffectArgs,
) error {
	switch args.Effect {
	case jobs.SlackEffectTriage:
		return s.syncTriage(ctx, conn, conv)
	case jobs.SlackEffectNote:
		return s.echoNote(ctx, conn, conv, args.MessageID)
	case jobs.SlackEffectDrafted:
		return s.postDraft(ctx, conn, conv, args.RunID)
	case jobs.SlackEffectAutoReply:
		return s.autoReply(ctx, conv)
	case jobs.SlackEffectJudge:
		return s.judgeDraft(ctx, conv, args.MessageID)
	default:
		return nil
	}
}
