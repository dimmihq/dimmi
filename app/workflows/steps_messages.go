/*
|--------------------------------------------------------------------------
| Customer Steps
|--------------------------------------------------------------------------
|
| Workflow messages are automatic: they never count as anyone's reply
| and never bill. Each carries a dedupe key made of the run and the
| step, so a retried job finds the message it already wrote instead of
| sending it twice. Questions read the same on every channel: numbered
| choices the customer answers with a number, a label, or their own
| words. A conversation that already got its hourly share of workflow
| messages gets no more, and the run goes on without them.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The outcomes of customer steps.
*/
const (
	outcomeSent          = "sent"
	outcomeUnreachable   = "unreachable"
	outcomeMessageBudget = "message_budget"
)

/*
executor runs a step; resumer hands a waiting step what it waited for.
*/
type (
	executor func(ctx context.Context, st *state, s workflowspec.Step) (outcome, error)
	resumer  func(ctx context.Context, st *state, s workflowspec.Step, in Input) (outcome, error)
)

/*
executors returns the executor of every step kind the engine runs.
*/
func executors() map[string]executor {
	return map[string]executor{
		workflowspec.KindMessage: runMessage, workflowspec.KindButtons: runButtons,
		workflowspec.KindCollect: runCollect, workflowspec.KindBranch: runBranch, workflowspec.KindWait: runWait,
		workflowspec.KindAssign: runAssign, workflowspec.KindUnassign: runUnassign, workflowspec.KindTag: runTag,
		workflowspec.KindUntag: runUntag, workflowspec.KindPriority: runPriority, workflowspec.KindNote: runNote,
		workflowspec.KindUpdateContact: runUpdateContact, workflowspec.KindSnooze: runSnooze,
		workflowspec.KindClose: runClose, workflowspec.KindDimmi: runDimmi,
		workflowspec.KindReplyTime: runReplyTime, workflowspec.KindClassify: runClassify,
		workflowspec.KindRunWorkflow: runRunWorkflow, workflowspec.KindHTTPRequest: runHTTPRequest,
	}
}

/*
resumers returns the resumer of every step kind that waits.
*/
func resumers() map[string]resumer {
	return map[string]resumer{
		workflowspec.KindButtons: resumeButtons, workflowspec.KindCollect: resumeCollect,
		workflowspec.KindWait: resumeWait, workflowspec.KindDimmi: resumeDimmi,
		workflowspec.KindRunWorkflow: resumeRunWorkflow,
	}
}

/*
settings decodes a step's settings.
*/
func settings[T any](s workflowspec.Step) T {
	var out T
	_ = json.Unmarshal(s.Settings, &out)

	return out
}

/*
runMessage sends a message.
*/
func runMessage(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	m := settings[workflowspec.MessageSettings](s)

	text, err := st.render(ctx, m.Text)
	if err != nil {
		return outcome{}, err
	}

	return st.send(ctx, s, text, "")
}

/*
send writes a workflow message to the customer and hands it to the
conversation's channel. The suffix tells repeated messages of one
step apart, such as asking again.
*/
func (st *state) send(ctx context.Context, s workflowspec.Step, text, suffix string) (outcome, error) {
	if over, err := st.overMessageBudget(ctx); over || err != nil {
		return outcome{outcome: outcomeMessageBudget, status: "skipped"}, err
	}

	route, err := st.route(ctx)
	if errors.Is(err, exceptions.ErrContactUnreachable) || errors.Is(err, exceptions.ErrChannelUnsupported) {
		return outcome{outcome: outcomeUnreachable, status: "skipped", acted: true}, nil
	}

	if err != nil {
		return outcome{}, err
	}

	key := "wf:" + st.run.ID.String() + ":" + s.ID + suffix
	meta, _ := json.Marshal(map[string]any{"workflow": map[string]any{
		"workflow_id": models.PublicID(models.PrefixWorkflow, st.run.WorkflowID),
		"run_id":      models.PublicID(models.PrefixWorkflowRun, st.run.ID), "step_id": s.ID,
	}})

	msg, conv, err := st.writer().Append(ctx, st.conv, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: text, BodyFormat: new("markdown"), DeliveryStatus: &route.Status,
		DedupeKey: &key, Metadata: meta, Automatic: true,
	})
	if err != nil {
		return outcome{}, fmt.Errorf("send workflow message: %w", err)
	}

	st.conv = conv
	if err := st.e.Delivery.Dispatch(ctx, st.tx, msg); err != nil {
		return outcome{}, err
	}

	return outcome{outcome: outcomeSent, acted: true, detail: map[string]any{
		"message_id": models.PublicID(models.PrefixMessage, msg.ID),
	}}, nil
}

/*
route finds how a message reaches the conversation's customer.
*/
func (st *state) route(ctx context.Context) (conversations.Route, error) {
	conn, err := conversations.Connection(ctx, st.q, st.conv)
	if err != nil {
		return conversations.Route{}, err
	}

	contact, err := st.q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ContactID,
	})
	if err != nil {
		return conversations.Route{}, fmt.Errorf("find contact: %w", err)
	}

	return st.e.Delivery.Plan(conn, contact)
}

/*
overMessageBudget reports whether the conversation got its hourly share
of workflow messages.
*/
func (st *state) overMessageBudget(ctx context.Context) (bool, error) {
	sent, err := st.q.CountBotMessagesSince(ctx, models.CountBotMessagesSinceParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: &st.conv.ID, Since: st.now().Add(-hour),
	})
	if err != nil {
		return false, fmt.Errorf("count workflow messages: %w", err)
	}

	return int(sent) >= st.e.Limits.MessagesPerHour, nil
}

/*
runButtons asks a question with numbered choices and waits for the
answer.
*/
func runButtons(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	b := settings[workflowspec.ButtonsSettings](s)

	text, err := st.render(ctx, b.Text)
	if err != nil {
		return outcome{}, err
	}

	sent, err := st.send(ctx, s, text+"\n\n"+choiceList(s), "")
	if err != nil || sent.outcome != outcomeSent {
		return sent, err
	}

	return outcome{wait: st.inputWait(b.Timeout), acted: true, detail: sent.detail}, nil
}

/*
choiceList is the numbered list of a step's buttons.
*/
func choiceList(s workflowspec.Step) string {
	var lines []string
	for i, p := range buttonPaths(s) {
		lines = append(lines, strconv.Itoa(i+1)+". "+p.Label)
	}

	return strings.Join(lines, "\n")
}

/*
buttonPaths returns the step's button paths in order.
*/
func buttonPaths(s workflowspec.Step) []workflowspec.Path {
	var out []workflowspec.Path
	for _, p := range s.Paths {
		if p.Outcome == "" {
			out = append(out, p)
		}
	}

	return out
}

/*
inputWait is a wait for the customer's next message, with its timeout.
*/
func (st *state) inputWait(timeout string) *waitState {
	w := &waitState{Kind: waitInput}
	if d, err := workflowspec.ParseDuration(timeout); err == nil && timeout != "" {
		until := st.now().Add(d)
		w.Until = &until
	}

	return w
}

/*
resumeButtons takes the path of the choice the answer names.
*/
func resumeButtons(ctx context.Context, st *state, s workflowspec.Step, in Input) (outcome, error) {
	if in.Kind == InputTimer {
		return fixedOrEnd(s, workflowspec.OutcomeTimeout, EndNoChoice), nil
	}

	if p, ok := matchChoice(buttonPaths(s), in.Text); ok {
		return outcome{path: p.ID, outcome: "button", detail: map[string]any{"path_id": p.ID}}, nil
	}

	b := settings[workflowspec.ButtonsSettings](s)
	if typed, ok := workflowspec.PathFor(s, workflowspec.OutcomeTyped); ok && workflowspec.On(b.AllowTyping, true) {
		return outcome{path: typed.ID, outcome: workflowspec.OutcomeTyped}, nil
	}

	if st.wait.Retried {
		return outcome{end: EndNoChoice, outcome: "no_choice"}, nil
	}

	retry := b.RetryText
	if retry == "" {
		retry = "Please reply with one of the numbers above."
	}

	if _, err := st.send(ctx, s, retry+"\n\n"+choiceList(s), ":retry"); err != nil {
		return outcome{}, err
	}

	w := st.wait
	w.Retried = true

	return outcome{wait: &w, outcome: "invalid_answer"}, nil
}

/*
fixedOrEnd takes the step's fixed path for the outcome, or ends the run
for the reason when the step has none.
*/
func fixedOrEnd(s workflowspec.Step, fixed, reason string) outcome {
	if p, ok := workflowspec.PathFor(s, fixed); ok {
		return outcome{path: p.ID, outcome: fixed}
	}

	return outcome{end: reason, outcome: fixed}
}
