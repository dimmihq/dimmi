/*
|--------------------------------------------------------------------------
| The Agent In Slack
|--------------------------------------------------------------------------
|
| How the agent behaves in a room is the room's to say. It may wait a
| while before answering, always or only while the team is at work, so
| a teammate gets the first word. While it writes, the thread shows
| Slack's own working state, or the eyes reaction where that state is
| not available: in direct messages, in channels shared with another
| organization, and wherever Slack refuses it. Stopping the agent from
| Slack hands the conversation to the team.
|
| What customers and teammates say about an answer is kept on its run:
| solved, a request for a human, or a plain good or bad. When a linked
| teammate joins a thread the agent stayed quiet in, they are told why,
| once, and nobody else sees it.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long the channel remembers how a working state was shown, and that
a thread's silence was explained.
*/
const (
	workingFor   = time.Hour
	explainedFor = 30 * 24 * time.Hour
)

/*
The ways a working state is shown.
*/
const (
	workingSession  = "session"
	workingReaction = "reaction"
)

/*
silenceReasons say, for each reason the agent skips, why it stayed
quiet.
*/
func silenceReasons() map[string]string {
	return map[string]string{
		"human_replied":          "a teammate answered first",
		"human_replied_in_delay": "a teammate answered while it waited",
		"human_active":           "the teammate it is assigned to is working on it",
		"assigned":               "the conversation is assigned to a teammate",
		"handed_off":             "the conversation was handed to the team",
		"not_addressed":          "the message was aimed at someone else",
		"auto_reply":             "the message was only a thank-you or a reaction",
		"agent_disabled":         "the agent is off for this channel",
		"schedule":               "it only answers outside office hours",
		"spend_cap":              "the month's AI budget is used up",
	}
}

/*
AgentDelay is how long the conversation's room holds the agent back so
a teammate can answer first, and whether it only does so inside office
hours.
*/
func (s *Slack) AgentDelay(ctx context.Context, workspaceID, conversationID ids.UUID) (time.Duration, bool) {
	ctx = tenancy.With(ctx, workspaceID.String())

	conv, err := s.findConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return 0, false
	}

	room, ok, err := s.roomOfConversation(ctx, conv)
	if err != nil || !ok {
		return 0, false
	}

	seconds := min(max(numberOf(&room, settingAgentDelay), 0), maxDelaySeconds)

	return time.Duration(seconds) * time.Second, settingOf(&room, settingAgentDelayWhen) == whenInsideHours
}

/*
startWorking shows the agent working in the thread: Slack's working
state where it can, the eyes reaction otherwise.
*/
func (s *Slack) startWorking(ctx context.Context, target workingTarget, conv models.Conversation) {
	channel, ts, _ := slackdriver.SplitMessageID(target.message.ChannelMessageID)
	thread, ok := threadOf(conv)
	room, inRoom, err := s.roomOfConversation(ctx, conv)

	how := workingReaction
	if ok && !thread.Direct() && inRoom && err == nil && !room.IsSharedExternal {
		if api, err := s.api(ctx, target.conn); err == nil &&
			api.SetSession(ctx, channel, thread.RootTS, slackdriver.SessionProcessing) == nil {
			how = workingSession
		}
	}

	if how == workingReaction {
		_ = s.Driver.StartWorking(ctx, target.conn, target.message)
	}

	_ = s.Cache.Set(ctx, workingKey(channel, ts), []byte(how), workingFor)
}

/*
stopWorking takes the working state off the thread, the way it was
shown.
*/
func (s *Slack) stopWorking(ctx context.Context, target workingTarget, conv models.Conversation) {
	channel, ts, _ := slackdriver.SplitMessageID(target.message.ChannelMessageID)

	how, err := s.Cache.Get(ctx, workingKey(channel, ts))
	if err == nil && string(how) == workingSession {
		if thread, ok := threadOf(conv); ok && !thread.Direct() {
			if api, err := s.api(ctx, target.conn); err == nil {
				_ = api.SetSession(ctx, channel, thread.RootTS, slackdriver.SessionActive)
			}
		}

		return
	}

	_ = s.Driver.StopWorking(ctx, target.conn, target.message)
}

/*
workingKey remembers how the working state of a message was shown.
*/
func workingKey(channel, ts string) string {
	return "slack:working:" + channel + ":" + ts
}

/*
onSessionStopped hands the conversation to the team when someone
stopped the agent in Slack.
*/
func (s *Slack) onSessionStopped(ctx context.Context, ev SlackEvent) (bool, error) {
	channel, root := ev.Event.ChannelID(), ev.Event.ThreadTS
	if channel == "" || root == "" {
		return false, nil
	}

	key := slackdriver.DirectThreadKey(channel)
	if room, ok, err := s.watchedRoom(ctx, ev.Connection, channel); err != nil {
		return false, err
	} else if ok {
		key = slackdriver.RoomThreadKey(room.ID, root)
	}

	conv, found, err := s.threadConversation(ctx, ev.Connection, key)
	if err != nil || !found {
		return false, err
	}

	if api, err := s.api(ctx, ev.Connection); err == nil {
		_ = api.SetSession(ctx, channel, root, slackdriver.SessionActive)
	}

	return true, s.handOff(ctx, conv)
}

/*
onFeedback records a good or bad on the agent's answer.
*/
func (s *Slack) onFeedback(ctx context.Context, in SlackInteraction) (bool, error) {
	publicID := in.Action.ActionID[len(slackdriver.FeedbackPrefix()):]
	verdict := in.Action.Value
	if verdict != "good" && verdict != "bad" {
		return false, nil
	}

	found, ok, err := s.answerOf(ctx, in.Connection, publicID)
	if err != nil || !ok {
		return false, err
	}

	return true, s.feedbackOn(ctx, found.message, verdict)
}

/*
feedbackOn records the verdict on the run that wrote the answer.
*/
func (s *Slack) feedbackOn(ctx context.Context, answer models.Message, verdict string) error {
	var meta struct {
		AI struct {
			RunID string `json:"run_id"`
		} `json:"ai"`
	}

	_ = json.Unmarshal(answer.Metadata, &meta)

	runID, err := ids.Parse(meta.AI.RunID)
	if err != nil {
		return ignoreUnreadable(err)
	}

	return s.recordFeedback(ctx, answer.WorkspaceID, runID, verdict)
}

/*
controlFeedback is the verdict an answer control stands for.
*/
func controlFeedback(control string) string {
	if control == contracts.ControlSolved {
		return "solved"
	}

	return "handoff"
}

/*
explainSilence tells a linked teammate who joins the thread why the
agent stayed quiet in it, once per thread.
*/
func (s *Slack) explainSilence(ctx context.Context, in ingestion) {
	conv, found, err := s.threadConversation(ctx, in.conn, in.threadKey)
	if err != nil || !found {
		return
	}

	var runs []models.AiRun

	err = s.read(ctx, func(q *models.Queries) error {
		var err error

		runs, err = q.ListConversationRuns(ctx, models.ListConversationRunsParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: &conv.ID, PageLimit: 1,
		})

		return err
	})
	if err != nil || len(runs) == 0 || runs[0].Status != "skipped" || runs[0].SkipReason == nil {
		return
	}

	why, ok := silenceReasons()[*runs[0].SkipReason]
	if !ok {
		return
	}

	first, err := s.Cache.Add(ctx, "slack:silence:"+conv.ID.String(), []byte{1}, explainedFor)
	if err != nil || !first {
		return
	}

	if api, err := s.api(ctx, in.conn); err == nil {
		_ = api.Ephemeral(ctx, slackdriver.Notice{
			Channel: in.ev.ChannelID(), User: in.author.user.ID, ThreadTS: in.ev.ThreadTS,
			Text: "dimmi stayed quiet in this thread: " + why + ".",
		})
	}
}
