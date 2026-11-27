package managers

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
tracedDecider sends a model call span for each call to the decision
model, as tracedLLM does for language models: the questions asked by
name, the model that answered and its tokens. The state the questions
are about stays out, like the decision itself. A call made outside any
agent run runs inside a background run, named after its task or
"decision".
*/
type tracedDecider struct {
	contracts.Decider

	telemetry *telemetry.Telemetry
	system    string
}

/*
TracedDecider wraps the decision model, or returns it as it is when no
tracker is configured or there is no model.
*/
func TracedDecider(t *telemetry.Telemetry, d contracts.Decider, system string) contracts.Decider {
	if t == nil || d == nil {
		return d
	}

	return &tracedDecider{Decider: d, telemetry: t, system: system}
}

/*
Decide runs the call inside a model call span.
*/
func (d *tracedDecider) Decide(
	ctx context.Context, req contracts.DecisionRequest,
) (*contracts.DecisionResponse, error) {
	var spans callSpans
	if !telemetry.InAgent(ctx) {
		task := telemetry.Task(ctx)
		if task == "" {
			task = "decision"
		}
		var run telemetry.AISpan
		ctx, run = d.telemetry.StartAI(ctx, telemetry.OpAgent, "background "+task)
		run.Set("gen_ai.agent.name", "background: "+task)
		run.Set("gen_ai.agent.id", "background."+task)
		spans.run = &run
	}

	ctx, span := d.telemetry.StartAI(ctx, telemetry.OpChat, "decide "+d.system)
	spans.chat = span
	defer spans.end()

	span.Set("gen_ai.system", d.system)
	span.Set("dimmi.decision.questions", strings.Join(slices.Sorted(maps.Keys(req.Questions)), ","))

	res, err := d.Decider.Decide(ctx, req)
	if err != nil {
		spans.fail()
		span.Set("error.type", telemetry.ErrorType(err))

		return nil, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
	}

	if res != nil {
		if res.Model != "" {
			span.Set("gen_ai.request.model", res.Model)
			span.Set("gen_ai.response.model", res.Model)
		}
		span.Set("gen_ai.usage.input_tokens", res.Usage.InputTokens)
		span.Set("gen_ai.usage.output_tokens", res.Usage.OutputTokens)
		span.Set("gen_ai.usage.total_tokens", res.Usage.InputTokens+res.Usage.OutputTokens)
	}

	return res, err //nolint:wrapcheck // the span is transparent; callers read the driver's error
}
