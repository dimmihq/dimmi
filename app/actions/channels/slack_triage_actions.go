/*
|--------------------------------------------------------------------------
| Triage Card Buttons
|--------------------------------------------------------------------------
|
| Every button on a triage card acts as the teammate who pressed it,
| through the same writer the inbox uses, so the timeline, the events
| and the card itself follow as if they had acted in dimmi. Take it is
| first come, first served: a second teammate is told who took it. Hand
| back to AI lets the agent answer again, on the record. Draft help
| article turns a conversation the team solved into an article draft to
| review; nothing is published by itself.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxArticleTitle is the longest title an article draft gets.
*/
const maxArticleTitle = 150

/*
onTriageAction handles a press on a triage card.
*/
func (s *Slack) onTriageAction(ctx context.Context, in SlackInteraction) (bool, error) {
	publicID, verb, ok := slackdriver.ParseTeamActionID(slackdriver.TriagePrefix(), in.Action.ActionID)
	if !ok {
		return false, nil
	}

	conv, ok, err := s.conversationByPublicID(ctx, in.Connection, publicID)
	if err != nil || !ok || verb == slackdriver.TriageOpen {
		return ok, err
	}

	who, allowed, err := s.teammateFor(ctx, in, conv)
	if err != nil || !allowed {
		return true, err
	}

	if err := s.triageVerb(ctx, in, triageAct{who: who, conv: conv, verb: verb}); err != nil {
		return true, err
	}

	return true, s.Queue.Push(ctx, effectOf(conv.WorkspaceID, conv.ID, nil, jobs.SlackEffectTriage))
}

/*
triageAct is a press on a triage card: who pressed, on which
conversation, and what the button says.
*/
type triageAct struct {
	who  teammate
	conv models.Conversation
	verb string
}

/*
triageVerb does what the button says.
*/
func (s *Slack) triageVerb(ctx context.Context, in SlackInteraction, act triageAct) error {
	who, conv := act.who, act.conv

	switch act.verb {
	case slackdriver.TriageTake:
		return s.take(ctx, in, who, conv)
	case slackdriver.TriageStatus:
		return s.moveTo(ctx, who, conv, in.Action.Selected.Value)
	case slackdriver.TriageSnooze:
		return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
			until, err := conversations.SnoozeUntil(w.Now, nil, conversations.PresetTomorrow, time.UTC)
			if err != nil {
				return err
			}

			_, _, err = w.ChangeStatus(ctx, conv, conversations.StatusChange{
				Status: string(enums.ConversationStatusSnoozed), Until: until,
			})

			return ignoreRefusal(err)
		})
	case slackdriver.TriageUrgent:
		return s.markUrgent(ctx, who, conv)
	case slackdriver.TriageAIBack:
		return s.handBack(ctx, who, conv)
	case slackdriver.TriageArticle:
		return s.draftArticle(ctx, in, who, conv)
	default:
		return nil
	}
}

/*
take assigns the conversation to the teammate, unless someone else took
it first.
*/
func (s *Slack) take(ctx context.Context, in SlackInteraction, who teammate, conv models.Conversation) error {
	if conv.AssigneeMemberID != nil && *conv.AssigneeMemberID != who.memberID {
		s.tell(ctx, in, s.memberName(ctx, conv)+" took it.")

		return nil
	}

	return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		_, _, err := w.Assign(ctx, conv, conversations.Assignment{Member: &who.memberID, Team: conv.AssigneeTeamID})

		return err
	})
}

/*
memberName names the conversation's assignee.
*/
func (s *Slack) memberName(ctx context.Context, conv models.Conversation) string {
	name := "Someone else"

	_ = s.read(ctx, func(q *models.Queries) error {
		member, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
			WorkspaceID: conv.WorkspaceID, ID: *conv.AssigneeMemberID,
		})
		if err == nil && member.User.Name != "" {
			name = member.User.Name
		}

		return err
	})

	return name
}

/*
moveTo opens or closes the conversation.
*/
func (s *Slack) moveTo(ctx context.Context, who teammate, conv models.Conversation, status string) error {
	if status != string(enums.ConversationStatusOpen) && status != string(enums.ConversationStatusClosed) {
		return nil
	}

	return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		_, _, err := w.ChangeStatus(ctx, conv, conversations.StatusChange{
			Status: status, Reason: conversations.ReasonManual,
		})

		return ignoreRefusal(err)
	})
}

/*
markUrgent makes the conversation urgent.
*/
func (s *Slack) markUrgent(ctx context.Context, who teammate, conv models.Conversation) error {
	return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		urgent := enums.ConversationPriorityUrgent
		_, _, err := w.SetAttributes(ctx, conv, conversations.Attributes{Priority: &urgent})

		return err
	})
}

/*
handBack lets the agent answer the conversation again, on the record.
*/
func (s *Slack) handBack(ctx context.Context, who teammate, conv models.Conversation) error {
	if conv.AiState != "handed_off" {
		return nil
	}

	return s.asTeammate(ctx, who, func(ctx context.Context, w *conversations.Writer) error {
		err := w.Q.ReactivateConversationAi(ctx, models.ReactivateConversationAiParams{
			WorkspaceID: conv.WorkspaceID, ID: conv.ID, Now: w.Now,
		})
		if err != nil {
			return fmt.Errorf("hand back to the agent: %w", err)
		}

		return w.Record(ctx, conv, "ai.resumed", map[string]any{"source": "slack"})
	})
}

/*
draftArticle turns the conversation into a help article draft for the
team to review.
*/
func (s *Slack) draftArticle(ctx context.Context, in SlackInteraction, who teammate, conv models.Conversation) error {
	if s.Articles == nil {
		return nil
	}

	var (
		center models.HelpCenter
		draft  articleText
	)

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		if center, err = q.FindDefaultHelpCenter(ctx, conv.WorkspaceID); err != nil {
			return err
		}

		draft, err = articleOf(ctx, q, conv)

		return err
	})
	if err != nil {
		s.tell(ctx, in, "dimmi could not draft an article: the workspace has no help center yet.")

		return ignoreMissing(err)
	}

	_, err = s.Articles.Create(ctx, helpcenters.ArticleInput{
		Membership: who.membership, HelpCenterID: center.ID, Title: &draft.title, Body: &draft.body,
	})
	if err != nil {
		s.tell(ctx, in, "dimmi could not draft an article from this conversation.")

		return ignoreRefusal(err)
	}

	s.tell(ctx, in, "Drafted the help article “"+draft.title+"” for review in dimmi. Nothing is published yet.")

	return nil
}

/*
articleText is an article draft's title and body.
*/
type articleText struct {
	title string
	body  string
}

/*
articleOf writes the conversation up as a question and the team's
answer.
*/
func articleOf(ctx context.Context, q *models.Queries, conv models.Conversation) (articleText, error) {
	messages, err := q.ListMessages(ctx, models.ListMessagesParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, RowLimit: 50,
	})
	if err != nil {
		return articleText{}, err
	}

	var asked, answers []string

	for _, m := range messages {
		if m.Type != string(enums.MessageTypeMessage) || m.Body == nil {
			continue
		}

		switch m.AuthorType {
		case string(enums.AuthorTypeContact):
			asked = append(asked, *m.Body)
		case string(enums.AuthorTypeMember):
			answers = append(answers, *m.Body)
		}
	}

	question := strings.Join(asked, "\n\n")
	title := strings.TrimSpace(strings.SplitN(question, "\n", 2)[0])

	if conv.Subject != nil && *conv.Subject != "" {
		title = *conv.Subject
	}

	return articleText{
		title: cutRunes(cmpTitle(title), maxArticleTitle),
		body:  "## Question\n\n" + question + "\n\n## Answer\n\n" + strings.Join(answers, "\n\n"),
	}, nil
}

/*
cmpTitle names an article whose question had no words.
*/
func cmpTitle(title string) string {
	if title == "" {
		return "Answer from a support conversation"
	}

	return title
}

/*
cutRunes returns at most limit runes of the text.
*/
func cutRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit])
}

/*
ignoreRefusal drops a refusal of a move the conversation cannot make,
such as snoozing a closed one or an article the help center refuses:
the card shows where things stand.
*/
func ignoreRefusal(err error) error {
	if _, refused := errors.AsType[*fault.Error](err); refused {
		return nil
	}

	return err
}
