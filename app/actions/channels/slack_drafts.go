/*
|--------------------------------------------------------------------------
| Slack Drafts
|--------------------------------------------------------------------------
|
| A room set to draft has the agent write for the team, not for the
| customer. Each answer lands as a draft in the conversation's triage
| thread, or, with no triage card, as a note only the room's linked
| teammates see, with Send, Edit and Discard. Send posts it to the
| customer as the teammate who pressed it, through their own Slack
| account when they connected it; Edit lets them change it first;
| Discard drops it and says so on the run. A draft is settled once.
|
| Every reply a teammate sends while a draft waits is compared with the
| draft, so the room's settings can say how often the agent agreed with
| the team before anyone lets it answer by itself.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
draftSettledFor is how long a settled draft stays settled against a
second press.
*/
const draftSettledFor = 30 * 24 * time.Hour

/*
DraftsFor reports whether the agent drafts in the conversation's room.
*/
func (s *Slack) DraftsFor(ctx context.Context, workspaceID, conversationID ids.UUID) bool {
	ctx = tenancy.With(ctx, workspaceID.String())

	conv, err := s.findConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return false
	}

	room, ok, err := s.roomOfConversation(ctx, conv)

	return err == nil && ok && settingOf(&room, settingAgentMode) == agentModeDraft
}

/*
Drafted queues the run's draft to be shown to the team once the run
commits.
*/
func (s *Slack) Drafted(ctx context.Context, tx *database.Tx, d contracts.AgentDraft) error {
	args := effectOf(d.WorkspaceID, d.ConversationID, nil, jobs.SlackEffectDrafted)
	args.RunID = &d.RunID

	return s.Queue.PushTx(ctx, tx, args)
}

/*
postDraft shows the run's draft to the team.
*/
func (s *Slack) postDraft(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	runID *ids.UUID,
) error {
	run, ok, err := s.draftRun(ctx, conv.WorkspaceID, runID)
	if err != nil || !ok || run.ConversationID == nil || *run.ConversationID != conv.ID {
		return err
	}

	card, err := s.triageCard(ctx, conv, models.ChannelRoom{})
	if err != nil {
		return err
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	draft := slackdriver.DraftCard{
		RunID: models.PublicID(models.PrefixAiRun, run.ID), Customer: card.Customer, Body: *run.DraftBody,
	}

	post, found, err := s.triagePost(ctx, conv)
	if err != nil {
		return err
	}

	if !found {
		return s.draftToTeammates(ctx, conn, conv, draft)
	}

	ts, err := api.PostTeamCard(ctx, slackdriver.DraftCardOf(post.ExternalChannel, post.ExternalTs, draft))
	if err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.CreateChannelConversationPost(ctx, models.CreateChannelConversationPostParams{
			WorkspaceID: conv.WorkspaceID, ChannelConnectionID: conn.ID, ConversationID: conv.ID, Kind: "draft",
			ExternalChannel: post.ExternalChannel, ExternalTs: ts, RunID: &run.ID, Now: s.Clock.Now(),
		})

		return err
	})
}

/*
draftToTeammates shows the draft in the customer's thread to each
linked teammate alone.
*/
func (s *Slack) draftToTeammates(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	draft slackdriver.DraftCard,
) error {
	room, ok, err := s.roomOfConversation(ctx, conv)
	if err != nil || !ok {
		return err
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	thread, _ := threadOf(conv)

	var links []models.ChannelMemberLink

	err = s.read(ctx, func(q *models.Queries) error {
		links, err = q.ListChannelMemberLinks(ctx, models.ListChannelMemberLinksParams{
			WorkspaceID: conv.WorkspaceID, ChannelConnectionID: conn.ID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list linked teammates: %w", err)
	}

	for _, link := range links {
		_, user, _ := strings.Cut(link.ExternalUserID, ":")
		_ = api.EphemeralTeamCard(ctx, user, slackdriver.DraftCardOf(room.ExternalID, thread.RootTS, draft))
	}

	return nil
}

/*
draftRun returns the run with its draft, reporting false for a run
without one.
*/
func (s *Slack) draftRun(ctx context.Context, workspaceID ids.UUID, runID *ids.UUID) (models.AiRun, bool, error) {
	if runID == nil {
		return models.AiRun{}, false, nil
	}

	var run models.AiRun

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		run, err = q.FindRun(ctx, models.FindRunParams{WorkspaceID: workspaceID, ID: *runID})

		return err
	})
	if err != nil {
		return run, false, ignoreMissing(err)
	}

	return run, run.DraftBody != nil && run.ConversationID != nil, nil
}

/*
onDraftAction handles Send, Edit and Discard on a draft.
*/
func (s *Slack) onDraftAction(ctx context.Context, in SlackInteraction) (bool, error) {
	publicID, verb, ok := slackdriver.ParseTeamActionID(slackdriver.DraftPrefix(), in.Action.ActionID)
	if !ok {
		return false, nil
	}

	d, ok, err := s.draftOf(ctx, in.Connection, publicID)
	if err != nil || !ok {
		return ok, err
	}

	who, allowed, err := s.teammateFor(ctx, in, d.conv)
	if err != nil || !allowed {
		return true, err
	}

	switch verb {
	case slackdriver.DraftSend:
		d.text = *d.run.DraftBody

		return true, s.sendDraft(ctx, in, who, d)
	case slackdriver.DraftEdit:
		if utf8.RuneCountInString(*d.run.DraftBody) > slackdriver.DraftEditorLimit {
			s.tell(ctx, in, "This draft is too long to edit in Slack. Edit it in dimmi, or send it as it is.")

			return true, nil
		}

		api, err := s.api(ctx, in.Connection)
		if err != nil {
			return true, err
		}

		return true, api.OpenDraftEditor(ctx, in.Interaction.TriggerID, publicID, *d.run.DraftBody)
	case slackdriver.DraftDiscard:
		return true, s.discardDraft(ctx, in, who, d)
	default:
		return false, nil
	}
}

/*
onDraftEdited sends the draft as the teammate changed it.
*/
func (s *Slack) onDraftEdited(ctx context.Context, in SlackInteraction) (bool, error) {
	view := in.Interaction.View
	if view == nil {
		return false, nil
	}

	d, ok, err := s.draftOf(ctx, in.Connection, view.PrivateMetadata)
	if err != nil || !ok {
		return ok, err
	}

	text := strings.TrimSpace(view.Value(slackdriver.DraftEditorBlock, slackdriver.DraftEditorInput))
	if text == "" {
		return true, nil
	}

	who, allowed, err := s.teammateFor(ctx, in, d.conv)
	if err != nil || !allowed {
		return true, err
	}

	d.text = text

	return true, s.sendDraft(ctx, in, who, d)
}

/*
pendingDraft is a draft a teammate acts on: its run, its conversation,
and the text to send.
*/
type pendingDraft struct {
	run  models.AiRun
	conv models.Conversation
	text string
}

/*
draftOf finds the draft a button names, on the connection, reporting
false for any other.
*/
func (s *Slack) draftOf(ctx context.Context, conn models.ChannelConnection, publicID string) (pendingDraft, bool,
	error,
) {
	runID, err := s.IDs.Decode(string(models.PrefixAiRun), publicID)
	if err != nil {
		return pendingDraft{}, false, ignoreUnreadable(err)
	}

	run, ok, err := s.draftRun(ctx, conn.WorkspaceID, &runID)
	if err != nil || !ok {
		return pendingDraft{}, false, err
	}

	conv, err := s.findConversation(ctx, conn.WorkspaceID, *run.ConversationID)
	if err != nil {
		return pendingDraft{}, false, ignoreMissing(err)
	}

	return pendingDraft{run: run, conv: conv}, conv.ChannelConnectionID == conn.ID, nil
}

/*
settle marks the draft settled, reporting false when it already was,
and tells the teammate so.
*/
func (s *Slack) settle(ctx context.Context, in SlackInteraction, d pendingDraft) bool {
	first, err := s.Cache.Add(ctx, "slack:draft-settled:"+d.run.ID.String(), []byte{1}, draftSettledFor)
	if err == nil && !first {
		s.tell(ctx, in, "This draft was already sent or discarded.")
	}

	return err == nil && first
}

/*
sendDraft posts the text to the customer as the teammate and settles
the draft.
*/
func (s *Slack) sendDraft(ctx context.Context, in SlackInteraction, who teammate, d pendingDraft) error {
	if !s.settle(ctx, in, d) {
		return nil
	}

	meta, _ := json.Marshal(map[string]any{"ai": map[string]any{"from_draft": d.run.ID.String()}})
	if err := s.replyAs(ctx, who, teamReply{conv: d.conv, body: d.text, meta: meta}); err != nil {
		return err
	}

	return s.closeDraft(ctx, in.Connection, d, settled{status: "sent", outcome: "Sent by " + who.name})
}

/*
discardDraft drops the draft and records the discard on its run.
*/
func (s *Slack) discardDraft(ctx context.Context, in SlackInteraction, who teammate, d pendingDraft) error {
	if !s.settle(ctx, in, d) {
		return nil
	}

	if err := s.recordFeedback(ctx, d.run.WorkspaceID, d.run.ID, "discarded"); err != nil {
		return err
	}

	return s.closeDraft(ctx, in.Connection, d, settled{status: "discarded", outcome: "Discarded by " + who.name})
}

/*
settled is how a draft ended: its post's status and the words shown.
*/
type settled struct {
	status  string
	outcome string
}

/*
closeDraft shows the draft settled where it was posted.
*/
func (s *Slack) closeDraft(ctx context.Context, conn models.ChannelConnection, d pendingDraft, how settled) error {
	var post models.ChannelConversationPost

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		post, err = q.FindDraftPost(ctx, models.FindDraftPostParams{WorkspaceID: d.run.WorkspaceID, RunID: &d.run.ID})
		if err != nil {
			return err
		}

		_, err = q.SetChannelConversationPostStatus(ctx, models.SetChannelConversationPostStatusParams{
			Status: how.status, Now: s.Clock.Now(), WorkspaceID: post.WorkspaceID, ID: post.ID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("settle draft: %w", err)
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	return api.UpdateTeamCard(ctx, post.ExternalTs,
		slackdriver.SettledDraftOf(post.ExternalChannel, how.outcome, *d.run.DraftBody))
}

/*
recordFeedback records a verdict on the run's answer.
*/
func (s *Slack) recordFeedback(ctx context.Context, workspaceID, runID ids.UUID, feedback string) error {
	now := s.Clock.Now()

	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.RecordRunFeedback(ctx, models.RecordRunFeedbackParams{
			Feedback: &feedback, Now: &now, WorkspaceID: workspaceID, ID: runID,
		})
		if err != nil {
			return fmt.Errorf("record feedback: %w", err)
		}

		return nil
	})
}

/*
judgeDraft compares the conversation's waiting draft with the reply a
teammate sent. A reply sent from the draft unchanged agrees without
asking anyone; without a judge, or a model to judge with, the draft
stays unjudged.
*/
func (s *Slack) judgeDraft(ctx context.Context, conv models.Conversation, messageID *ids.UUID) error {
	if messageID == nil {
		return nil
	}

	var (
		reply models.Message
		run   models.AiRun
	)

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		if reply, err = q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
			WorkspaceID: conv.WorkspaceID, ID: *messageID,
		}); err != nil {
			return err
		}

		run, err = q.FindUnjudgedDraft(ctx, models.FindUnjudgedDraftParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: &conv.ID,
		})

		return err
	})
	if err != nil || reply.AuthorType != string(enums.AuthorTypeMember) || run.DraftBody == nil {
		return ignoreMissing(err)
	}

	agreed := strings.TrimSpace(body(reply)) == strings.TrimSpace(*run.DraftBody)
	if !agreed && s.Judge == nil {
		return nil
	}

	if !agreed {
		agreed, err = s.Judge.Agree(ctx, DraftComparison{
			WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, Draft: *run.DraftBody, Reply: body(reply),
		})
		if err != nil {
			return noVerdict(err)
		}
	}

	return s.read(ctx, func(q *models.Queries) error {
		return q.SetRunShadowAgreed(ctx, models.SetRunShadowAgreedParams{
			ShadowAgreed: &agreed, Now: s.Clock.Now(), WorkspaceID: run.WorkspaceID, ID: run.ID,
		})
	})
}

/*
ShadowScore returns how many of the room's latest judged drafts agreed
with the reply the teammate sent, and how many were judged.
*/
func (s *Slack) ShadowScore(ctx context.Context, workspaceID, roomID ids.UUID) (agreed, judged int64, err error) {
	ctx = tenancy.With(ctx, workspaceID.String())

	err = s.read(ctx, func(q *models.Queries) error {
		score, err := q.ShadowScore(ctx, models.ShadowScoreParams{
			WorkspaceID: workspaceID, KeyPrefix: slackdriver.RoomThreadPrefix(roomID), RowLimit: shadowWindow,
		})
		agreed, judged = score.Agreed, score.Judged

		return err
	})
	if err != nil {
		return 0, 0, fmt.Errorf("read shadow score: %w", err)
	}

	return agreed, judged, nil
}

/*
noVerdict drops a judge's failure: without a model to ask, or one that
failed, the draft simply stays unjudged.
*/
func noVerdict(error) error {
	return nil
}
