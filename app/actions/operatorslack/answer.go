/*
|--------------------------------------------------------------------------
| Answering In The Thread
|--------------------------------------------------------------------------
|
| The thread shows a placeholder at once, then Operator's answer in its
| place, with a quiet line naming what it looked at. When Operator can
| not answer, because it is off, has no model or the budget is spent,
| the placeholder says why instead. A follow-up in the same Slack thread
| continues the same Operator thread, which stays private to the
| teammate who started it.
|
*/

package operatorslack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/ai/operator"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
answer asks Operator as the teammate and answers in the Slack thread.
*/
func (o *Operator) answer(ctx context.Context, in Asked, who slackteam.Teammate) error {
	api, err := o.Bot(ctx, in.Connection)
	if err != nil {
		return err
	}

	channel, root := in.Event.ChannelID(), rootOf(in.Event)

	ts, err := api.PostReply(ctx, channel, root, placeholder)
	if err != nil {
		return fmt.Errorf("post operator placeholder: %w", err)
	}

	reply, err := o.ask(ctx, in, who)
	if err != nil {
		return api.UpdateCard(ctx, channel, ts, slackdriver.Card{Text: failure(err), Lines: []string{failure(err)}})
	}

	if err := api.UpdateCard(ctx, channel, ts, answerCard(reply.Message)); err != nil {
		return fmt.Errorf("post operator answer: %w", err)
	}

	for _, c := range reply.Changesets {
		if c.Status != "pending" {
			continue
		}

		if _, err := api.PostCard(ctx, channel, root, ChangesetCard(c, who.Name)); err != nil {
			return fmt.Errorf("post operator changeset: %w", err)
		}
	}

	return nil
}

/*
ask finds or starts the Operator thread of the Slack thread and asks on
it.
*/
func (o *Operator) ask(ctx context.Context, in Asked, who slackteam.Teammate) (operator.Answer, error) {
	thread, err := o.thread(ctx, in, who)
	if err != nil {
		return operator.Answer{}, err
	}

	return o.Threads.Ask(ctx, operatorchat.AskInput{Membership: who.Membership, ThreadID: thread, Body: question(in)})
}

/*
thread returns the Operator thread of the Slack thread, starting one
about the conversation the message links to, or the whole workspace.
*/
func (o *Operator) thread(ctx context.Context, in Asked, who slackteam.Teammate) (ids.UUID, error) {
	channel, root := in.Event.ChannelID(), rootOf(in.Event)

	var found models.OperatorSlackThread

	err := o.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		found, err = models.New(tx).FindOperatorSlackThread(ctx, models.FindOperatorSlackThreadParams{
			WorkspaceID: in.Connection.WorkspaceID, ChannelConnectionID: in.Connection.ID, SlackChannel: channel,
			ThreadTs: root,
		})

		return err
	})
	if err == nil {
		return found.OperatorThreadID, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return ids.UUID{}, fmt.Errorf("find operator slack thread: %w", err)
	}

	started, err := o.Threads.Open(ctx, who.Membership, o.mentioned(in.Event.Text), true)
	if err != nil {
		return ids.UUID{}, err
	}

	return started.Thread.ID, o.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).CreateOperatorSlackThread(ctx, models.CreateOperatorSlackThreadParams{
			WorkspaceID: in.Connection.WorkspaceID, OperatorThreadID: started.Thread.ID,
			ChannelConnectionID: in.Connection.ID, SlackChannel: channel, ThreadTs: root, Now: o.Clock.Now(),
		})

		return err
	})
}

/*
failure explains why Operator could not answer.
*/
func failure(err error) string {
	if errors.Is(err, exceptions.ErrNotFound) {
		return "This thread belongs to another teammate's talk with Operator. Start a new thread to ask your own."
	}

	if f, ok := errors.AsType[*fault.Error](err); ok {
		return f.Message()
	}

	return "Operator could not answer just now. Try again in a moment."
}

/*
answerCard is Operator's answer as a card, with what it looked at.
*/
func answerCard(m models.OperatorMessage) slackdriver.Card {
	card := slackdriver.Card{Text: slackdriver.Truncate(m.Body, 3000), Markdown: slackdriver.ToSlack(m.Body)}

	var steps []operator.Step
	_ = json.Unmarshal(m.Steps, &steps)

	var looked []string

	for _, s := range steps {
		if s.Summary != "" {
			looked = append(looked, slackdriver.Escape(s.Summary))
		}
	}

	if len(looked) > 0 {
		card.Context = "Operator: " + strings.Join(looked, " · ")
	}

	return card
}
