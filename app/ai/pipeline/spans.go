package pipeline

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The kinds of agent run the error tracker tells apart, beside a
simulation.
*/
const (
	runConversation = "conversation"
	runFollowUp     = "action_follow_up"
)

/*
conversationOf is the conversation a run is grouped under on the error
tracker. A simulated chat has no conversation, so its turns share the
chat's id, and a single simulated turn stands alone under its run's.
*/
func conversationOf(st *run, kind string) ids.UUID {
	if kind != simulationKind {
		return st.env.Conversation.ID
	}
	if st.sim != nil && st.sim.in.ChatID != (ids.UUID{}) {
		return st.sim.in.ChatID
	}
	if st.rec != nil {
		return st.rec.Run.ID
	}
	return ids.UUID{}
}

/*
tracedRun runs one agent's turn inside an agent run span named after
the agent as the workspace sees it, so the error tracker shows the run
with its model and tool calls beneath it and groups the runs of one
conversation. A run that fails is marked failed; a run that hands the
conversation to another agent ends with a handoff span naming both.
*/
func (r *Runner) tracedRun(
	ctx context.Context, st *run, kind string, work func(context.Context) (*Result, error),
) (*Result, error) {
	ctx, span := r.Telemetry.StartAI(ctx, telemetry.OpAgent, "invoke_agent "+st.env.Agent.Name)
	defer span.End()

	span.Set("gen_ai.agent.name", st.env.Agent.Name)
	span.Set("gen_ai.agent.id", st.env.Agent.ID.String())
	if id := conversationOf(st, kind); id != (ids.UUID{}) {
		span.Set("gen_ai.conversation.id", id.String())
	}
	span.Set("dimmi.workspace_id", st.env.Agent.WorkspaceID.String())
	span.Set("dimmi.run.kind", kind)
	span.Set("dimmi.simulation", kind == simulationKind)
	if st.rec != nil {
		span.Set("dimmi.run_id", st.rec.Run.ID.String())
	}

	res, err := work(ctx)
	switch {
	case err != nil || res == nil || res.Outcome == outcomeError:
		span.Fail()
	case res.Outcome == outcomeTransferred && st.transfer != nil:
		r.tracedHandoff(ctx, st)
	}
	if res != nil {
		span.Set("dimmi.outcome", res.Outcome)
	}

	return res, err
}

/*
tracedHandoff records that the agent handed the conversation to
another agent. The tracker draws the handoff from the agent the span
names to the next agent run after it.
*/
func (r *Runner) tracedHandoff(ctx context.Context, st *run) {
	from, to := st.env.Agent, st.transfer.to
	_, span := r.Telemetry.StartAI(ctx, telemetry.OpHandoff, "handoff from "+from.Name+" to "+to.Name)
	span.Set("gen_ai.agent.name", from.Name)
	span.Set("gen_ai.agent.id", from.ID.String())
	span.Set("dimmi.handoff.to.name", to.Name)
	span.Set("dimmi.handoff.to.id", to.ID.String())
	span.End()
}

/*
callTool runs one tool the model asked for inside a tool span that
records what it was given and what it returned. A tool that fails is
marked failed with its error type; one that answers no records how. A
tool that panics is marked failed and the panic goes on.
*/
func (r *Runner) callTool(ctx context.Context, st *run, call contracts.ToolCall) (string, bool) {
	ctx, span := r.Telemetry.StartAI(ctx, telemetry.OpTool, "execute_tool "+call.Name)
	defer span.End()

	span.Set("gen_ai.tool.name", call.Name)
	span.Set("gen_ai.tool.input", string(call.Arguments))

	result, refused := r.runTool(ctx, st, call)
	span.Set("gen_ai.tool.output", result)

	return result, refused
}

/*
traceAction marks the tool span with how an action call ended: failed
with its code, or the no it answered with.
*/
func traceAction(ctx context.Context, reply actionkit.CallReply) {
	switch {
	case reply.Failure != "":
		telemetry.FailTool(ctx, reply.Failure)
	case reply.Pending:
		telemetry.ToolOutcome(ctx, "pending_approval")
	case reply.Awaiting:
		telemetry.ToolOutcome(ctx, "awaiting_confirmation")
	case !reply.Succeeded:
		telemetry.ToolOutcome(ctx, "refused")
	}
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
