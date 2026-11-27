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
model that answered and the tokens it used.
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
	defer span.End()

	res, err := l.LLM.Chat(ctx, req)
	if err != nil {
		span.Fail()

		return nil, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
	}

	answered(span, res.Text, res.ToolCalls)
	record(span, res.Model, res.Usage)

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
		span.Fail()
		span.End()

		return nil, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
	}

	return &tracedStream{ChatStream: stream, span: span}, nil
}

func (l *tracedLLM) start(ctx context.Context, req contracts.ChatRequest) (context.Context, telemetry.AISpan) {
	ctx, span := l.telemetry.StartAI(ctx, "gen_ai.chat", "chat "+req.Model)
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

	return ctx, span
}

/*
tracedStream gathers what the model writes, records it with the usage
event on the span, and ends the span on the stream's last event, its
failure or its close.
*/
type tracedStream struct {
	contracts.ChatStream

	span  telemetry.AISpan
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
		s.span.Fail()
		s.end()
	case ev.Kind == contracts.StreamEventTextDelta:
		s.text.WriteString(ev.Text)
	case ev.Kind == contracts.StreamEventToolCallDone && ev.ToolCall != nil:
		s.calls = append(s.calls, *ev.ToolCall)
	case ev.Kind == contracts.StreamEventUsage && ev.Usage != nil:
		record(s.span, ev.Model, *ev.Usage)
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
		answered(s.span, s.text.String(), s.calls)
		s.span.End()
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
