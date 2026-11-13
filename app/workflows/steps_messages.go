/*
|--------------------------------------------------------------------------
| Customer Steps
|--------------------------------------------------------------------------
|
| Workflow messages are automatic: they never count as anyone's reply
| and never bill. Each carries a dedupe key made of the run and the
| step, so a retried job finds the message it already wrote instead of
| sending it twice. A question's answers are buttons the customer taps
| on a channel that draws them, and a numbered list the customer answers
| with a number, a label, or their own words anywhere else. A conversation that already got its hourly share of workflow
| messages gets no more, and the run goes on without them.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
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
	outcomeArticleGone   = "article_gone"
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
	all := map[string]executor{
		workflowspec.KindMessage: runMessage, workflowspec.KindArticle: runArticle,
		workflowspec.KindButtons: runButtons,
		workflowspec.KindCollect: runCollect, workflowspec.KindBranch: runBranch, workflowspec.KindWait: runWait,
		workflowspec.KindAssign: runAssign, workflowspec.KindUnassign: runUnassign, workflowspec.KindTag: runTag,
		workflowspec.KindUntag: runUntag, workflowspec.KindPriority: runPriority, workflowspec.KindNote: runNote,
		workflowspec.KindUpdateContact: runUpdateContact, workflowspec.KindSnooze: runSnooze,
		workflowspec.KindClose: runClose, workflowspec.KindDimmi: runDimmi,
		workflowspec.KindReplyTime: runReplyTime, workflowspec.KindClassify: runClassify,
		workflowspec.KindRunWorkflow: runRunWorkflow, workflowspec.KindHTTPRequest: runHTTPRequest,
	}
	maps.Copy(all, ticketExecutors())

	return all
}

/*
resumers returns the resumer of every step kind that waits.
*/
func resumers() map[string]resumer {
	return map[string]resumer{
		workflowspec.KindButtons: resumeButtons, workflowspec.KindCollect: resumeCollect,
		workflowspec.KindWait: resumeWait, workflowspec.KindDimmi: resumeDimmi,
		workflowspec.KindRunWorkflow: resumeRunWorkflow, workflowspec.KindHTTPRequest: resumeHTTPRequest,
		workflowspec.KindClassify: resumeClassify,
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
written is the body a message carries and the metadata beside it: on a
channel that draws them, the answers travel beside the body; anywhere
else they are written into it as a numbered list.
*/
func (st *state) written(out outgoing, can contracts.ChannelCapabilities) (string, []byte) {
	workflow := map[string]any{
		"workflow_id": models.PublicID(models.PrefixWorkflow, st.run.WorkflowID),
		"run_id":      models.PublicID(models.PrefixWorkflowRun, st.run.ID), "step_id": out.step.ID,
	}

	body := out.text
	if len(out.choices) > 0 {
		if can.Rich {
			workflow["choices"] = out.choices
		} else {
			body = out.text + "\n\n" + numbered(labels(out.choices))
		}
	}

	maps.Copy(workflow, out.parts)
	meta, _ := json.Marshal(map[string]any{"workflow": workflow})

	return body, meta
}

/*
runArticle answers with a help article: the line the workflow wrote, if
any, and the article itself, which the messenger opens in place and
every other channel reads as a link.
*/
func runArticle(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	a := settings[workflowspec.ArticleSettings](s)

	id, ok := st.decode(models.PrefixArticle, a.Article)
	if !ok {
		return skipped(outcomeArticleGone), nil
	}

	article, found := st.article(ctx, id)
	if !found {
		return skipped(outcomeArticleGone), nil
	}

	text, err := st.render(ctx, a.Text)
	if err != nil {
		return outcome{}, err
	}

	return st.sendArticle(ctx, s, text, article)
}

/*
article reads the article the step names, false when it is no longer
there.
*/
func (st *state) article(ctx context.Context, id ids.UUID) (models.Article, bool) {
	row, err := st.q.FindArticle(ctx, models.FindArticleParams{WorkspaceID: st.conv.WorkspaceID, ID: id})

	return row, err == nil
}

/*
sendArticle writes the article into the conversation: its title and
summary as a link anyone can follow, and the article beside the body so
the messenger opens it in place.
*/
func (st *state) sendArticle(
	ctx context.Context, s workflowspec.Step, text string, article models.Article,
) (outcome, error) {
	url := st.articleURL(ctx, article)
	card := map[string]any{"slug": article.Slug, "title": article.Title}
	if article.Description != nil && *article.Description != "" {
		card["summary"] = *article.Description
	}

	if url != "" {
		card["url"] = url
	}

	rich, err := st.richChannel(ctx)
	if err != nil {
		return outcome{}, err
	}

	body := text
	if !rich {
		link := article.Title
		if url != "" {
			link = "[" + article.Title + "](" + url + ")"
		}

		body = link
		if text != "" {
			body = text + "\n\n" + link
		}
	}

	return st.sendParts(ctx, outgoing{step: s, text: body, parts: map[string]any{"article": card}})
}

/*
richChannel reports whether the conversation's channel draws what a
message carries beside its body.
*/
func (st *state) richChannel(ctx context.Context) (bool, error) {
	route, err := st.route(ctx)
	if errors.Is(err, exceptions.ErrContactUnreachable) || errors.Is(err, exceptions.ErrChannelUnsupported) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return route.Capabilities.Rich, nil
}

/*
articleURL is where the article reads on the help center, empty when the
workspace has no address for it.
*/
func (st *state) articleURL(ctx context.Context, article models.Article) string {
	center, err := st.q.FindHelpCenter(ctx, models.FindHelpCenterParams{
		WorkspaceID: article.WorkspaceID, ID: article.HelpCenterID,
	})
	if err != nil {
		return ""
	}

	root := st.e.HelpCenters.Root(center)
	if root == "" {
		return ""
	}

	return root + "/articles/" + article.Slug
}

/*
send writes a workflow message to the customer and hands it to the
conversation's channel. The suffix tells repeated messages of one
step apart, such as asking again.
*/
func (st *state) send(ctx context.Context, s workflowspec.Step, text, suffix string) (outcome, error) {
	return st.sendParts(ctx, outgoing{step: s, text: text, suffix: suffix})
}

/*
sendChoices writes a message whose answers the customer picks from. A
channel that draws them as buttons carries them beside the body; anywhere
else they read as a numbered list the customer answers in their own
words.
*/
func (st *state) sendChoices(ctx context.Context, out outgoing) (outcome, error) {
	return st.sendParts(ctx, out)
}

/*
outgoing is one message a workflow sends: the step it belongs to, its
text, the suffix that tells repeated messages of one step apart, the
answers it offers and anything else the messenger draws beside it.
*/
type outgoing struct {
	step    workflowspec.Step
	text    string
	suffix  string
	parts   map[string]any
	choices []choice
}

/*
choice is one answer a question offers: the path it takes and the label
the customer reads.
*/
type choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

/*
sendParts writes the message and carries what the messenger draws beside
its body, such as a question's choices or an article.
*/
func (st *state) sendParts(ctx context.Context, out outgoing) (outcome, error) {
	s, suffix := out.step, out.suffix

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
	body, meta := st.written(out, route.Capabilities)

	msg, conv, err := st.writer().Append(ctx, st.conv, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: body, BodyFormat: new("markdown"), DeliveryStatus: &route.Status,
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

	parts := map[string]any{}
	if !workflowspec.On(b.AllowTyping, true) {
		parts["choices_only"] = true
	}

	sent, err := st.sendParts(ctx, outgoing{step: s, text: text, parts: parts, choices: choiceLabels(s)})
	if err != nil || sent.outcome != outcomeSent {
		return sent, err
	}

	return outcome{wait: st.inputWait(b.Timeout), acted: true, detail: sent.detail}, nil
}

/*
chosen returns the path a tapped button named, false when the answer
named none of this step's paths.
*/
func chosen(paths []workflowspec.Path, path string) (workflowspec.Path, bool) {
	if path == "" {
		return workflowspec.Path{}, false
	}

	for _, p := range paths {
		if p.ID == path {
			return p, true
		}
	}

	return workflowspec.Path{}, false
}

/*
choiceLabels are a step's buttons, in the order they were built: each
path and what it reads.
*/
func choiceLabels(s workflowspec.Step) []choice {
	var out []choice
	for _, p := range buttonPaths(s) {
		out = append(out, choice{ID: p.ID, Label: p.Label})
	}

	return out
}

/*
labels are the words of choices, for a channel that writes them out.
*/
func labels(choices []choice) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.Label
	}

	return out
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

	paths := buttonPaths(s)
	if p, ok := chosen(paths, in.Choice); ok {
		return outcome{path: p.ID, outcome: "button", detail: map[string]any{"path_id": p.ID}}, nil
	}

	if p, ok := matchChoice(paths, in.Text); ok {
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

	retrying := outgoing{step: s, text: retry, suffix: ":retry", choices: choiceLabels(s)}
	if _, err := st.sendChoices(ctx, retrying); err != nil {
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
