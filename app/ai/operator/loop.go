/*
|--------------------------------------------------------------------------
| Operator Loop
|--------------------------------------------------------------------------
|
| The model reads the thread and calls tools until it writes its answer.
| Each call streams: its text reaches the teammate as it is written, and
| each tool it runs shows as a step. After a set number of model calls
| or tool calls the tools are withdrawn, so the model has to answer with
| or tool calls the model is told to answer with what it has found. A
| routine may still close its run for a few calls more, so a run that
| read too much can still suggest what it found.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/ai/ledger"
	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
The bounds of one run: model calls, tool calls, and the answer's length.
*/
const (
	maxModelCalls   = 12
	maxToolCalls    = 40
	maxOutputTokens = 4096
)

/*
The bounds of a chat that took on building work by reading one of the
chat skills, such as designing a workflow: researching the workspace,
building, and trying each path takes far more than a question does.
*/
const (
	buildingModelCalls = 30
	buildingToolCalls  = 100
)

/*
streamed is what one model call produced.
*/
type streamed struct {
	text       string
	calls      []contracts.ToolCall
	usage      contracts.LLMUsage
	firstToken time.Duration
	requestID  string
	model      string
}

/*
converse runs model calls and tools until the model answers, and
returns the answer with how the run ended. Only the last call's text is
the answer: what the model writes before calling tools is a note among
the run's steps, where it was written.
*/
func (e *Engine) converse(ctx context.Context, s *session) (string, string) {
	e.openLayer(ctx, s)
	defer e.saveLayer(ctx, s)
	p := prompt{system: e.systemPrompt(s), messages: e.transcript(s)}
	p.newest = len(p.messages) - 1
	if s.layer != nil {
		p.system += "\n\n" + s.layer.set.Prompt()
		s.layer.system = p.system
	}
	for calls := 1; ; calls++ {
		spent, closing := e.bounds(s, calls)
		if spent {
			return e.conclude(ctx, s, p)
		}
		s.closing = closing
		out, err := e.generateWithRetry(ctx, s, p, e.toolSpecs(s))
		if err != nil {
			return strings.TrimSpace(out.text), statusOf(err)
		}
		if len(out.calls) == 0 {
			if strings.TrimSpace(out.text) == "" {
				return e.conclude(ctx, s, p)
			}
			return strings.TrimSpace(out.text), "done"
		}
		e.note(ctx, s, out.text)
		p.messages = append(p.messages, contracts.ChatMessage{
			Role: contracts.ChatRoleAssistant, Parts: textParts(out.text), ToolCalls: out.calls,
		})
		p.messages = append(p.messages, e.callTools(ctx, s, out.calls)...)
		remind(p.messages, e.running(s, calls))
		if ctx.Err() != nil {
			return "", statusOf(ctx.Err())
		}
	}
}

/*
The calls a routine keeps for closing its run once its reading is spent.
*/
const (
	closingModelCalls = 3
	closingToolCalls  = 8
)

/*
closes reports whether a tool closes a routine's run: suggesting,
saving what the next run needs, or saying nothing needs attention.
*/
func closes(name string) bool {
	switch name {
	case "report_nothing_to_change", "save_routine_state", "propose_changes", toolArticleEdit, toolArticleNew:
		return true
	}
	return false
}

/*
bounds reports where a run stands before a model call: spent when it
may call no more tools, closing when a routine has used up its reading
and may only close. The tools offered never change, so the provider's
prompt cache holds to the end; a closing run is refused the others.
*/
func (e *Engine) bounds(s *session, calls int) (spent, closing bool) {
	modelCalls, toolCalls := s.limits()
	if calls < modelCalls && s.toolCalls < toolCalls {
		return false, false
	}
	if s.routine == nil || calls >= modelCalls+closingModelCalls || s.toolCalls >= toolCalls+closingToolCalls {
		return true, false
	}
	return false, true
}

/*
The reading steps and turns a routine has left when it is first told so,
and from then on after every turn.
*/
const (
	lowToolCalls  = 20
	lowModelCalls = 5
)

/*
running is the note a routine's run reads once it is near its limits:
how many reading steps and turns it has left. It is empty for a chat,
for a run with plenty left, and for one already closing, which the
refusals speak for.
*/
func (e *Engine) running(s *session, calls int) string {
	if s.routine == nil {
		return ""
	}

	modelCalls, toolCalls := s.limits()
	steps, turns := toolCalls-s.toolCalls, modelCalls-calls

	if steps <= 0 || turns <= 0 || (steps > lowToolCalls && turns > lowModelCalls) {
		return ""
	}

	return fmt.Sprintf("[You have %d reading steps and %d turns left in this run. Read only what you still "+
		"need, then suggest and write your message.]", steps, turns)
}

/*
remind adds the note to the last tool result, where the model reads it
next. Nothing earlier in the conversation changes, so the provider's
prompt cache holds.
*/
func remind(messages []contracts.ChatMessage, note string) {
	if note == "" || len(messages) == 0 {
		return
	}

	last := &messages[len(messages)-1]
	last.Parts = append(last.Parts, textParts(note)...)
}

/*
conclude asks for the answer once the run may call no more tools. The
tools stay in the request so the prompt cache holds; any call the model
still makes is dropped, and a model that wrote nothing is asked once
more without them.
*/
func (e *Engine) conclude(ctx context.Context, s *session, p prompt) (string, string) {
	last := contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: textParts(prompts.Task(taskLastCall, nil))}
	p.messages = append(p.messages, last)
	out, err := e.generateWithRetry(ctx, s, p, e.toolSpecs(s))
	if err == nil && strings.TrimSpace(out.text) == "" {
		out, err = e.generateWithRetry(ctx, s, p, nil)
	}
	if err != nil {
		return strings.TrimSpace(out.text), statusOf(err)
	}
	return strings.TrimSpace(out.text), "done"
}

/*
noteLength is how much of a remark a step keeps.
*/
const noteLength = 400

/*
note keeps what the model wrote before calling tools as a step, so the
run's timeline shows it where it was said and the answer stays the last
thing written.
*/
func (e *Engine) note(ctx context.Context, s *session, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	e.addStep(ctx, s, Step{Tool: "note", Summary: truncate(text, noteLength), OK: true})
}

/*
routineRetries is how many more times a routine's run tries a model
call that failed before it produced anything.
*/
const routineRetries = 2

/*
generateWithRetry runs one model call. Nobody watches a routine's run,
so a call that timed out or hit an overloaded provider before writing
anything is tried again after a short wait instead of failing the run;
a teammate's question fails at once, since they can ask again.
*/
func (e *Engine) generateWithRetry(ctx context.Context, s *session, p prompt, tools []contracts.ToolSpec,
) (streamed, error) {
	out, err := e.generate(ctx, s, p, tools)
	for try := 1; try <= routineRetries && s.routine != nil && retriable(out, err); try++ {
		if e.RetryWait > 0 {
			select {
			case <-ctx.Done():
				return out, err
			case <-e.Clock.After(time.Duration(try) * e.RetryWait):
			}
		}
		s.failure = nil
		out, err = e.generate(ctx, s, p, tools)
	}
	return out, err
}

/*
retriable reports whether a failed call produced nothing and failed in
a way that may pass: a timeout, a dropped connection, throttling or an
overloaded or failing provider.
*/
func retriable(out streamed, err error) bool {
	if err == nil || out.text != "" || len(out.calls) > 0 {
		return false
	}
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	return ok && llmErr.Retryable()
}

/*
prompt is what a model call reads: the system prompt and the messages
so far.
*/
type prompt struct {
	system   string
	messages []contracts.ChatMessage
	newest   int
}

/*
generate streams one model call and records it.
*/
func (e *Engine) generate(ctx context.Context, s *session, p prompt, tools []contracts.ToolSpec) (streamed, error) {
	req := contracts.ChatRequest{
		Model: s.model, System: []contracts.SystemBlock{{Text: p.system, Cacheable: true}}, Messages: p.messages,
		Tools:           tools,
		MaxOutputTokens: maxOutputTokens, CacheHint: true, Effort: effortOf(s.settings),
		Timeout:  llmretry.StepTimeouts(e.Timeouts, llmretry.StepGenerate),
		Metadata: map[string]string{"session_id": s.thread.ID.String()},
	}
	if len(tools) > 0 {
		req.ToolChoice = contracts.ToolChoiceAuto
	}
	started := e.Clock.Now()
	out, err := e.stream(ctx, s, req, started)
	e.record(ctx, s, step{
		kind: enums.AIRunStepKindGenerate, name: s.model, started: started, firstToken: out.firstToken,
		response: out.text, usage: out.usage, ledger: ledger.Encode(ledger.Of(req, p.newest)), err: err,
		requestID: out.requestID, responseModel: out.model,
	})
	return out, err
}

/*
effortOf is how hard a call thinks: the settings' effort, medium when
unset.
*/
func effortOf(s Settings) contracts.Effort {
	if s.Effort == "" {
		return contracts.EffortMedium
	}
	return s.Effort
}

/*
stream runs one call and hands its text to the teammate as it comes.
*/
func (e *Engine) stream(ctx context.Context, s *session, req contracts.ChatRequest, started time.Time,
) (streamed, error) {
	ctx, cancel := context.WithTimeout(ctx, max(e.Timeouts.GenerateTotal, 30*time.Second)*2)
	defer cancel()
	st, err := s.llm.Stream(ctx, req)
	if err != nil {
		return streamed{}, err
	}
	defer func() { _ = st.Close() }()
	c := &collector{started: started}
	for {
		ev, err := st.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			c.out.text = c.text.String()
			return c.out, err
		}
		e.collect(ctx, s, c, ev)
	}
	c.out.text = c.text.String()
	return c.out, nil
}

/*
collector gathers one streaming call: what it produced so far, its text
and when it started.
*/
type collector struct {
	out     streamed
	text    strings.Builder
	started time.Time
}

/*
collect folds one stream event into the call's result, streaming text.
*/
func (e *Engine) collect(ctx context.Context, s *session, c *collector, ev contracts.StreamEvent) {
	switch ev.Kind {
	case contracts.StreamEventTextDelta:
		if c.out.firstToken == 0 {
			c.out.firstToken = max(e.Clock.Now().Sub(c.started), time.Millisecond)
		}
		c.text.WriteString(ev.Text)
		e.publishDelta(ctx, s, ev.Text)
	case contracts.StreamEventToolCallDone:
		if ev.ToolCall != nil {
			c.out.calls = append(c.out.calls, *ev.ToolCall)
		}
	case contracts.StreamEventUsage:
		if ev.Usage != nil {
			c.out.usage = *ev.Usage
		}
		c.out.requestID, c.out.model = ev.RequestID, ev.Model
	case contracts.StreamEventDone:
		return
	}
}

/*
transcript is the thread so far followed by the new question.
*/
func (e *Engine) transcript(s *session) []contracts.ChatMessage {
	var out []contracts.ChatMessage
	for _, m := range s.history {
		if strings.TrimSpace(m.Body) == "" {
			continue
		}
		out = append(out, contracts.ChatMessage{Role: speaker(m), Parts: textParts(m.Body)})
	}
	if len(out) > 0 {
		out[len(out)-1].Cacheable = true
	}
	question := contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: textParts(s.question)}
	if s.looking != "" {
		question.Parts = append(question.Parts, contracts.ContentPart{Text: s.looking})
	}
	if s.sub == nil {
		question.Parts = append(question.Parts, contracts.ContentPart{Text: writeReminder()})
	}
	if s.layer != nil {
		if note := toolset.Note(s.layer.earlier, s.layer.suggested); note != "" {
			question.Parts = append(question.Parts, contracts.ContentPart{Text: note})
		}
	}
	return append(out, question)
}

/*
speaker is the chat role of a stored turn: Operator's answers speak as
the assistant, the teammate's questions as the user.
*/
func speaker(m models.OperatorMessage) contracts.ChatRole {
	switch m.Role {
	case roleAssistant:
		return contracts.ChatRoleAssistant
	default:
		return contracts.ChatRoleUser
	}
}

/*
textParts wraps text as message parts, none when it is empty.
*/
func textParts(text string) []contracts.ContentPart {
	if text == "" {
		return nil
	}
	return []contracts.ContentPart{{Text: text}}
}
