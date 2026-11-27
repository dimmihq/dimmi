package managers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
tracedLLM sends a span for each model call to the error tracker: what
the model was asked, with its tools, and what it answered, with the
model that answered and the tokens it used. A call made outside any
agent run, by a background task, runs inside a run of its own named
after the task, so the tracker never lists it under an unnamed agent.
*/
type tracedLLM struct {
	contracts.LLM

	telemetry *telemetry.Telemetry
}

/*
traced wraps the connection, or returns it as it is when no tracker is
configured.
*/
func traced(t *telemetry.Telemetry, llm contracts.LLM) contracts.LLM {
	if t == nil {
		return llm
	}

	return &tracedLLM{LLM: llm, telemetry: t}
}

/*
Unwrap returns the connection beneath the spans.
*/
func (l *tracedLLM) Unwrap() contracts.LLM { return l.LLM }

/*
Chat runs the completion inside a model call span.
*/
func (l *tracedLLM) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	ctx, span := l.start(ctx, req)
	defer span.end()

	res, err := l.LLM.Chat(ctx, req)
	if err != nil {
		span.fail()

		return nil, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
	}

	answered(span.chat, res.Text, res.ToolCalls)
	record(span.chat, res.Model, res.Usage)

	return res, nil
}

/*
Stream runs the completion inside a model call span that ends when the
stream does.
*/
func (l *tracedLLM) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	ctx, span := l.start(ctx, req)

	stream, err := l.LLM.Stream(ctx, req)
	if err != nil {
		span.fail()
		span.end()

		return nil, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
	}

	return &tracedStream{ChatStream: stream, span: span}, nil
}

/*
callSpans are a model call's span and, for a call made outside any
agent run, the background run around it.
*/
type callSpans struct {
	run  *telemetry.AISpan
	chat telemetry.AISpan
}

func (c callSpans) fail() {
	c.chat.Fail()
	if c.run != nil {
		c.run.Fail()
	}
}

func (c callSpans) end() {
	c.chat.End()
	if c.run != nil {
		c.run.End()
	}
}

/*
backgroundTask names the background task a model call made outside any
agent run belongs to: the one the context names, else the response
schema the call asks for, else "model_call".
*/
func backgroundTask(ctx context.Context, req contracts.ChatRequest) string {
	task := telemetry.Task(ctx)
	if task == "" && req.Schema != nil {
		task = req.Schema.Name
	}
	if task == "" {
		task = "model_call"
	}

	return task
}

func (l *tracedLLM) start(ctx context.Context, req contracts.ChatRequest) (context.Context, callSpans) {
	var spans callSpans
	if !telemetry.InAgent(ctx) {
		task := backgroundTask(ctx, req)
		var run telemetry.AISpan
		ctx, run = l.telemetry.StartAI(ctx, telemetry.OpAgent, "background "+task)
		run.Set("gen_ai.agent.name", "background: "+task)
		run.Set("gen_ai.agent.id", "background."+task)
		spans.run = &run
	}

	ctx, span := l.telemetry.StartAI(ctx, telemetry.OpChat, "chat "+req.Model)
	spans.chat = span
	span.Set("gen_ai.system", string(contracts.UpstreamOf(l.LLM, req.Model)))
	span.Set("gen_ai.request.model", req.Model)
	span.Set("gen_ai.request.messages", asked(req))

	if len(req.Tools) > 0 {
		names := make([]string, len(req.Tools))
		for i, tool := range req.Tools {
			names[i] = tool.Name
		}

		span.Set("gen_ai.request.available_tools", jsonText(names))
	}

	return ctx, spans
}

/*
tracedStream gathers what the model writes, records it with the usage
event on the span, and ends the span on the stream's last event, its
failure or its close.
*/
type tracedStream struct {
	contracts.ChatStream

	span  callSpans
	text  strings.Builder
	calls []contracts.ToolCall
	ended bool
}

/*
Next reads the stream's next event.
*/
func (s *tracedStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	ev, err := s.ChatStream.Next(ctx)

	switch {
	case errors.Is(err, io.EOF):
		s.end()
	case err != nil:
		s.span.fail()
		s.end()
	case ev.Kind == contracts.StreamEventTextDelta:
		s.text.WriteString(ev.Text)
	case ev.Kind == contracts.StreamEventToolCallDone && ev.ToolCall != nil:
		s.calls = append(s.calls, *ev.ToolCall)
	case ev.Kind == contracts.StreamEventUsage && ev.Usage != nil:
		record(s.span.chat, ev.Model, *ev.Usage)
	}

	return ev, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
}

/*
Close ends the span and releases the stream.
*/
func (s *tracedStream) Close() error {
	s.end()

	return s.ChatStream.Close() //nolint:wrapcheck // the span is transparent; callers read the driver's error
}

func (s *tracedStream) end() {
	if !s.ended {
		s.ended = true
		answered(s.span.chat, s.text.String(), s.calls)
		s.span.end()
	}
}

/*
message is one message of a model call as the tracker shows it.
*/
type message struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []contracts.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

/*
asked returns the system prompt and the messages of the request as the
JSON the tracker reads.
*/
func asked(req contracts.ChatRequest) string {
	messages := make([]message, 0, len(req.Messages)+1)

	if len(req.System) > 0 {
		blocks := make([]string, len(req.System))
		for i, block := range req.System {
			blocks[i] = block.Text
		}

		messages = append(messages, message{Role: "system", Content: strings.Join(blocks, "\n\n")})
	}

	for _, m := range req.Messages {
		parts := make([]string, len(m.Parts))
		for i, part := range m.Parts {
			parts[i] = part.Text
		}

		messages = append(messages, message{
			Role: string(m.Role), Content: strings.Join(parts, ""), ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID,
		})
	}

	return jsonText(messages)
}

/*
answered puts what the model wrote and the tools it called on the span.
*/
func answered(span telemetry.AISpan, text string, calls []contracts.ToolCall) {
	if text != "" {
		span.Set("gen_ai.response.text", text)
	}

	if len(calls) > 0 {
		span.Set("gen_ai.response.tool_calls", jsonText(calls))
	}
}

/*
record puts the answering model and the tokens on the span.
*/
func record(span telemetry.AISpan, model string, usage contracts.LLMUsage) {
	if model != "" {
		span.Set("gen_ai.response.model", model)
	}

	span.Set("gen_ai.usage.input_tokens", usage.InputTokens)
	span.Set("gen_ai.usage.input_tokens.cached", usage.CachedInputTokens)
	span.Set("gen_ai.usage.output_tokens", usage.OutputTokens)
	span.Set("gen_ai.usage.output_tokens.reasoning", usage.ReasoningTokens)
	span.Set("gen_ai.usage.total_tokens", usage.InputTokens+usage.OutputTokens)
}

/*
jsonText returns the value as JSON text, or nothing when it cannot be
written.
*/
func jsonText(value any) string {
	out, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	return string(out)
}
