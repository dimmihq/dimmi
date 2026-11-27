package operator

import "context"

/*
converse runs model calls and tools until the model answers, inside an
agent run span, so the error tracker shows the run with its model and
tool calls beneath it.
*/
func (e *Engine) converse(ctx context.Context, s *session) (string, string) {
	ctx, span := e.Telemetry.StartAI(ctx, "gen_ai.invoke_agent", "operator")
	defer span.End()

	span.Set("gen_ai.agent.name", "operator")
	span.Set("gen_ai.conversation.id", s.thread.ID.String())
	span.Set("dimmi.workspace_id", s.thread.WorkspaceID.String())

	text, status := e.converseLoop(ctx, s)
	if status == "failed" {
		span.Fail()
	}

	return text, status
}

/*
runCall runs one call inside a tool span.
*/
func (e *Engine) runCall(ctx context.Context, s *session, p *pending) {
	ctx, span := e.Telemetry.StartAI(ctx, "gen_ai.execute_tool", "execute_tool "+p.call.Name)
	defer span.End()

	span.Set("gen_ai.tool.name", p.call.Name)

	e.execCall(ctx, s, p)
	if p.err != nil {
		span.Fail()
	}
}
