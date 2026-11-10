/*
|--------------------------------------------------------------------------
| Operator Loop
|--------------------------------------------------------------------------
|
| The model reads the thread and calls tools until it writes its answer.
| Each call streams: its text reaches the teammate as it is written, and
| each tool it runs shows as a step. After a set number of model calls
| or tool calls the tools are withdrawn, so the model has to answer with
| what it has found.
|
*/

package operator

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

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
}

/*
converse runs model calls and tools until the model answers, and
returns the answer with how the run ended.
*/
func (e *Engine) converse(ctx context.Context, s *session) (string, string) {
	p := prompt{system: e.systemPrompt(s), messages: e.transcript(s)}
	var written strings.Builder
	for calls := 1; ; calls++ {
		var tools []contracts.ToolSpec
		if calls < maxModelCalls && s.toolCalls < maxToolCalls {
			tools = e.toolSpecs(s)
		}
		out, err := e.generate(ctx, s, p, tools)
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
prompt is what a model call reads: the system prompt and the messages
so far.
*/
type prompt struct {
	system   string
	messages []contracts.ChatMessage
}

/*
generate streams one model call and records it.
*/
func (e *Engine) generate(ctx context.Context, s *session, p prompt, tools []contracts.ToolSpec) (streamed, error) {
	req := contracts.ChatRequest{
		Model: s.model, System: []contracts.SystemBlock{{Text: p.system}}, Messages: p.messages, Tools: tools,
		MaxOutputTokens: maxOutputTokens, CacheHint: true, Effort: contracts.EffortMedium,
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
		response: out.text, usage: out.usage, err: err,
	})
	return out, err
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
	return append(out, contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: textParts(s.question)})
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
