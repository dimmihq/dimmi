package managers

import (
	"context"
	"errors"
	"io"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
tracedLLM sends a span for each model call to the error tracker: the
model asked for and the one that answered, and the tokens it used. The
prompt and the answer are customers' words and are never sent.
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

	return ctx, span
}

/*
tracedStream records the usage event on the span and ends the span on
the stream's last event, its failure or its close.
*/
type tracedStream struct {
	contracts.ChatStream

	span  telemetry.AISpan
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
		s.span.End()
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
