package operator

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
operatorName and operatorID name Operator in the error tracker; a
subagent it spawns shows as its own agent beneath the run that spawned
it.
*/
const (
	operatorName = "Operator"
	operatorID   = "operator"
	subagentName = "Operator subagent"
	subagentID   = "operator.subagent"
)

/*
converse runs model calls and tools until the model answers, inside an
agent run span, so the error tracker shows the run with its model and
tool calls beneath it and groups the runs of one thread.
*/
func (e *Engine) converse(ctx context.Context, s *session) (string, string) {
	name, id := operatorName, operatorID
	if s.sub != nil {
		name, id = subagentName, subagentID
	}
	ctx, span := e.Telemetry.StartAI(ctx, telemetry.OpAgent, "invoke_agent "+name)
	defer span.End()

	span.Set("gen_ai.agent.name", name)
	span.Set("gen_ai.agent.id", id)
	span.Set("gen_ai.conversation.id", s.thread.ID.String())
	span.Set("dimmi.workspace_id", s.thread.WorkspaceID.String())

	text, status := e.converseLoop(ctx, s)
	if status == "failed" {
		span.Fail()
	}
	span.Set("dimmi.outcome", status)

	return text, status
}

/*
runCall runs one call inside a tool span that records what it was given
and what it returned. A call that fails is marked failed with its error
type; one held back by the run's reading budget is refused, not failed.
*/
func (e *Engine) runCall(ctx context.Context, s *session, p *pending) {
	ctx, span := e.Telemetry.StartAI(ctx, telemetry.OpTool, "execute_tool "+p.call.Name)
	defer span.End()

	span.Set("gen_ai.tool.name", p.call.Name)
	span.Set("gen_ai.tool.input", string(p.call.Arguments))

	e.execCall(ctx, s, p)
	span.Set("gen_ai.tool.output", p.out.text)

	switch {
	case p.err == nil:
	case errors.Is(p.err, errSpent):
		span.Set("tool.outcome", "refused")
	default:
		span.FailWith(callFailure(p))
	}
}

/*
callFailure names how a call failed: a tool the run does not have,
arguments that do not fit, or the error's own type.
*/
func callFailure(p *pending) string {
	switch {
	case !p.known && !p.layer:
		return "unknown_tool"
	case errors.Is(p.err, errBadArguments):
		return "invalid_arguments"
	}

	return telemetry.ErrorType(p.err)
}
