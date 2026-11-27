package pipeline

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
tracedRun runs one agent's turn inside an agent run span named after
the agent as the workspace sees it, so the error tracker shows the run
with its model and tool calls beneath it and groups the runs of one
conversation. A run that fails is marked failed; a run that hands the
conversation to another agent ends with a handoff span naming both.
*/
func (r *Runner) tracedRun(
	ctx context.Context, st *run, conversationID ids.UUID, work func(context.Context) (*Result, error),
) (*Result, error) {
	ctx, span := r.Telemetry.StartAI(ctx, telemetry.OpAgent, "invoke_agent "+st.env.Agent.Name)
	defer span.End()

	span.Set("gen_ai.agent.name", st.env.Agent.Name)
	span.Set("gen_ai.agent.id", st.env.Agent.ID.String())
	if conversationID != (ids.UUID{}) {
		span.Set("gen_ai.conversation.id", conversationID.String())
	}
	span.Set("dimmi.workspace_id", st.env.Agent.WorkspaceID.String())

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
