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
	span.Set("gen_ai.agent.id", s.agent.ID.String())
	span.Set("gen_ai.conversation.id", s.thread.ID.String())
	span.Set("dimmi.workspace_id", s.thread.WorkspaceID.String())

	text, status := e.converseLoop(ctx, s)
	if status == "failed" {
		span.Fail()
	}

	return text, status
}

/*
runCall runs one call inside a tool span that records what it was given
and what it returned.
*/
func (e *Engine) runCall(ctx context.Context, s *session, p *pending) {
	ctx, span := e.Telemetry.StartAI(ctx, "gen_ai.execute_tool", "execute_tool "+p.call.Name)
	defer span.End()

	span.Set("gen_ai.tool.name", p.call.Name)
	span.Set("gen_ai.tool.input", string(p.call.Arguments))

	e.execCall(ctx, s, p)
	span.Set("gen_ai.tool.output", p.out.text)

	if p.err != nil {
		span.Fail()
		span.Set("gen_ai.tool.error", p.err.Error())
	}
}
