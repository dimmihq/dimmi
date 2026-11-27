package operator

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/ai/toolset"
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
	span.Set("dimmi.run.kind", tracedKind(s))
	span.Set("dimmi.run_id", s.run.ID.String())

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
	case errors.Is(p.err, errBadArguments), errors.Is(p.err, toolset.ErrArguments):
		return "invalid_arguments"
	}

	return telemetry.ErrorType(p.err)
}

/*
unsaved records a write the run could not make and went on without, so
the error tracker shows it as a failure the run recovered from.
*/
func unsaved(ctx context.Context, kind string, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		telemetry.Recovered(ctx, kind, "skipped")
	}
}

/*
tracedKind tells Operator's runs apart on the error tracker: a subagent,
a routine's run, a triage, or a teammate's question as its run row
names it.
*/
func tracedKind(s *session) string {
	switch {
	case s.sub != nil:
		return "subagent"
	case s.routine != nil:
		return "routine"
	}

	return s.run.Kind
}
