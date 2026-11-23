/*
|--------------------------------------------------------------------------
| Operator Loop
|--------------------------------------------------------------------------
|
| The model reads the thread and calls tools until it writes its answer.
| Each call streams: its text reaches the teammate as it is written, and
| each tool it runs shows as a step. After a set number of model calls
| or tool calls the tools are withdrawn, so the model has to answer with
| what it has found. A routine keeps the tools that close a run for a
| few calls more, so a run that read too much can still suggest what it
| found.
|
*/

package operator

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/ai/ledger"
	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/llmretry"
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
returns the answer with how the run ended.
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
	var written strings.Builder
	for calls := 1; ; calls++ {
		tools := e.offered(s, calls)
		out, err := e.generateWithRetry(ctx, s, p, tools)
		written.WriteString(out.text)
		if err != nil {
			return strings.TrimSpace(written.String()), statusOf(err)
		}
		if len(out.calls) == 0 || len(tools) == 0 {
			return strings.TrimSpace(written.String()), "done"
		}
		if out.text != "" {
			written.WriteString("\n\n")
		}
		p.messages = append(p.messages, contracts.ChatMessage{
			Role: contracts.ChatRoleAssistant, Parts: textParts(out.text), ToolCalls: out.calls,
		})
		p.messages = append(p.messages, e.callTools(ctx, s, out.calls)...)
		if ctx.Err() != nil {
			return strings.TrimSpace(written.String()), statusOf(ctx.Err())
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
closingTools are the tools that close a routine's run: suggesting,
saving what the next run needs, and saying nothing needs attention.
*/
var closingTools = []string{"mark_noop", "propose_changes", "routine_set_state"}

/*
offered is the tools a model call may use: all of them within the run's
bounds, then for a routine only the closing ones for a few calls, then
none.
*/
func (e *Engine) offered(s *session, calls int) []contracts.ToolSpec {
	modelCalls, toolCalls := s.limits()
	if calls < modelCalls && s.toolCalls < toolCalls {
		return e.toolSpecs(s)
	}
	if s.routine == nil || calls >= modelCalls+closingModelCalls || s.toolCalls >= toolCalls+closingToolCalls {
		return nil
	}
	if s.layer != nil {
		return e.toolSpecs(s)
	}
	var out []contracts.ToolSpec
	for _, spec := range e.toolSpecs(s) {
		if slices.Contains(closingTools, spec.Name) {
			out = append(out, spec)
		}
	}
	return out
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
