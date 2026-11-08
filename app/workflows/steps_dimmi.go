/*
|--------------------------------------------------------------------------
| Letting dimmi Handle A Conversation
|--------------------------------------------------------------------------
|
| The agent speaks in a conversation a workflow holds only while this
| step is active. The step readies the agent, hands it the customer
| message already waiting, and waits for one of three things: the agent
| handing the conversation over, the customer confirming an answer
| solved it, or the customer going quiet after an answer. Each takes
| its own path; an empty path ends the run. Resolved means the customer
| said so: silence is never counted as solved.
|
| A workspace without a working agent never strands the customer: the
| step takes the handed-over path at once.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/resolution"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The default quiet waits: a chat customer is gone after minutes, an
email customer only after a day.
*/
const (
	quietChat  = "10m"
	quietEmail = "1d"
)

/*
runDimmi hands the conversation to the agent and waits.
*/
func runDimmi(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	d := settings[workflowspec.DimmiSettings](s)
	ready, err := st.agentReady(ctx)
	if err != nil || !ready {
		return handedOver(s, "agent_disabled"), err
	}

	if err := st.q.ReadyConversationAi(ctx, models.ReadyConversationAiParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID, Now: st.now(),
	}); err != nil {
		return outcome{}, fmt.Errorf("ready agent: %w", err)
	}

	w, err := st.agentWait(ctx, d)
	if err != nil {
		return outcome{}, err
	}

	if err := st.wakeAgent(ctx); err != nil {
		return outcome{}, err
	}

	return outcome{wait: w, outcome: "handling", acted: true}, nil
}

/*
agentReady reports whether the workspace has an agent that is on.
*/
func (st *state) agentReady(ctx context.Context) (bool, error) {
	if !st.e.AgentOn {
		return false, nil
	}

	agent, err := st.q.FindAgent(ctx, st.conv.WorkspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find agent: %w", err)
	}

	return agent.Enabled, nil
}

/*
agentWait is the wait of an active agent step: the guidance and limits
the agent reads on every turn, and the quiet timer.
*/
func (st *state) agentWait(ctx context.Context, d workflowspec.DimmiSettings) (*waitState, error) {
	instructions, err := st.render(ctx, d.Instructions)
	if err != nil {
		return nil, err
	}

	quiet, err := st.quietAfter(ctx, d.QuietAfter)
	if err != nil {
		return nil, err
	}

	now := st.now()
	until := now.Add(quiet)

	return &waitState{
		Kind: workflowspec.WaitAgent, Since: &now, Until: &until, Quiet: quiet.String(),
		Agent: &workflowspec.AgentHandling{
			Instructions: instructions, MaxReplies: d.MaxReplies,
			ConfirmResolution: workflowspec.On(d.ConfirmResolution, true),
		},
	}, nil
}

/*
quietAfter is how long a silent customer is waited for: the step's
own setting, or the channel's default.
*/
func (st *state) quietAfter(ctx context.Context, setting string) (time.Duration, error) {
	if setting == "" {
		subj, err := st.subject(ctx)
		if err != nil {
			return 0, err
		}

		setting = quietChat
		if subj.channel == "email" {
			setting = quietEmail
		}
	}

	d, err := workflowspec.ParseDuration(setting)
	if err != nil {
		return 0, fmt.Errorf("read quiet wait: %w", err)
	}

	return d, nil
}

/*
wakeAgent hands the agent the customer's message when it is the newest
one, so a customer who wrote before the step began gets an answer.
*/
func (st *state) wakeAgent(ctx context.Context) error {
	last, err := st.q.LastCustomerMessage(ctx, models.LastCustomerMessageParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID,
	})
	if errors.Is(err, database.ErrNotFound) || err == nil && last.AuthorType != string(enums.AuthorTypeContact) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find waiting message: %w", err)
	}

	return st.push(ctx, jobs.AnswerForWorkflowArgs{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, MessageID: last.ID, RunID: st.run.ID,
	})
}

/*
resumeDimmi takes the path of what ended the agent's turn, or keeps
waiting.
*/
func resumeDimmi(ctx context.Context, st *state, s workflowspec.Step, in Input) (outcome, error) {
	switch in.Kind {
	case InputAgentHandover:
		return handedOver(s, in.Reason), nil
	case InputMessage:
		return st.maybeResolved(ctx, s, in)
	case InputTimer:
		return st.maybeQuiet(ctx, s)
	}

	return st.keepWaiting(), nil
}

/*
handedOver takes the handed-over path, recording why.
*/
func handedOver(s workflowspec.Step, reason string) outcome {
	out := fixedOrEnd(s, workflowspec.OutcomeHandedOver, EndCompleted)
	out.detail, out.acted = map[string]any{"reason": reason}, true

	return out
}

/*
keepWaiting leaves the run waiting on the agent as it was.
*/
func (st *state) keepWaiting() outcome {
	w := st.wait

	return outcome{wait: &w}
}

/*
maybeResolved takes the resolved path when the customer confirms the
answer right before their message solved it.
*/
func (st *state) maybeResolved(ctx context.Context, s workflowspec.Step, in Input) (outcome, error) {
	if st.wait.Agent == nil || !st.wait.Agent.ConfirmResolution || in.MessageID == nil ||
		!resolution.Confirms(in.Text) {
		return st.keepWaiting(), nil
	}

	before, err := st.q.PublicAuthorBefore(ctx, models.PublicAuthorBeforeParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, MessageID: *in.MessageID,
	})
	if errors.Is(err, database.ErrNotFound) || err == nil && before != string(enums.AuthorTypeAgent) {
		return st.keepWaiting(), nil
	}

	if err != nil {
		return outcome{}, fmt.Errorf("find answer before: %w", err)
	}

	return st.resolved(ctx, s)
}

/*
resolved stands the agent down and takes the resolved path; with none,
it closes the conversation unless the step says not to.
*/
func (st *state) resolved(ctx context.Context, s workflowspec.Step) (outcome, error) {
	if err := st.standDown(ctx); err != nil {
		return outcome{}, err
	}

	out := fixedOrEnd(s, workflowspec.OutcomeResolved, EndCompleted)
	out.acted = true
	if out.path != "" || !workflowspec.On(settings[workflowspec.DimmiSettings](s).CloseOnResolved, true) {
		return out, nil
	}

	if _, err := st.changeStatus(ctx, conversations.StatusChange{
		Status: string(enums.ConversationStatusClosed),
	}); err != nil {
		return outcome{}, err
	}

	out.detail = map[string]any{"closed": true}

	return out, nil
}

/*
maybeQuiet takes the quiet path once the customer said nothing for the
quiet wait after the agent's last answer; until then it waits again,
counted from the latest message either of them wrote.
*/
func (st *state) maybeQuiet(ctx context.Context, s workflowspec.Step) (outcome, error) {
	quiet, err := time.ParseDuration(st.wait.Quiet)
	if err != nil || st.wait.Since == nil {
		return outcome{}, fmt.Errorf("read quiet wait: %w", errors.Join(err, errors.New("wait has no start")))
	}

	last, err := st.q.LastAgentAndCustomerMessages(ctx, models.LastAgentAndCustomerMessagesParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, Since: *st.wait.Since,
	})
	if err != nil {
		return outcome{}, fmt.Errorf("find last messages: %w", err)
	}

	answered := last.AgentAt.After(time.Unix(0, 0)) && !last.ContactAt.After(last.AgentAt)
	if answered && !st.now().Before(last.AgentAt.Add(quiet)) {
		return st.quiet(ctx, s)
	}

	return st.waitQuiet(latest(*st.wait.Since, last.AgentAt, last.ContactAt).Add(quiet), quiet), nil
}

/*
waitQuiet waits again until the moment, or a quiet wait from now when
that moment already passed.
*/
func (st *state) waitQuiet(at time.Time, quiet time.Duration) outcome {
	if !at.After(st.now()) {
		at = st.now().Add(quiet)
	}

	w := st.wait
	w.Until = &at

	return outcome{wait: &w}
}

/*
quiet stands the agent down and takes the quiet path.
*/
func (st *state) quiet(ctx context.Context, s workflowspec.Step) (outcome, error) {
	if err := st.standDown(ctx); err != nil {
		return outcome{}, err
	}

	out := fixedOrEnd(s, workflowspec.OutcomeQuiet, EndCompleted)
	out.acted = true

	return out, nil
}

/*
standDown stops the agent once the step is over.
*/
func (st *state) standDown(ctx context.Context) error {
	if err := st.q.StandDownConversationAi(ctx, models.StandDownConversationAiParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID, Now: st.now(),
	}); err != nil {
		return fmt.Errorf("stand agent down: %w", err)
	}

	return nil
}

/*
latest returns the latest of the moments.
*/
func latest(first time.Time, rest ...time.Time) time.Time {
	out := first
	for _, t := range rest {
		if t.After(out) {
			out = t
		}
	}

	return out
}
