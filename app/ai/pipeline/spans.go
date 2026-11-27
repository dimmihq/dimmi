package pipeline

import (
	"context"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
tracedTurn runs the agent's turn inside an agent run span, so the error
tracker shows the turn with its model and tool calls beneath it.
*/
func (r *Runner) tracedTurn(ctx context.Context, st *run, args Args) *decision {
	ctx, span := r.Telemetry.StartAI(ctx, "gen_ai.invoke_agent", "answer conversation")
	defer span.End()

	span.Set("gen_ai.agent.name", "agent")
	span.Set("gen_ai.conversation.id", args.ConversationID.String())
	span.Set("dimmi.workspace_id", args.WorkspaceID.String())

	return r.turn(ctx, st)
}

/*
callTool runs one tool the model asked for inside a tool span.
*/
func (r *Runner) callTool(ctx context.Context, st *run, call contracts.ToolCall) (string, bool) {
	ctx, span := r.Telemetry.StartAI(ctx, "gen_ai.execute_tool", "execute_tool "+call.Name)
	defer span.End()

	span.Set("gen_ai.tool.name", call.Name)

	return r.runTool(ctx, st, call)
}
