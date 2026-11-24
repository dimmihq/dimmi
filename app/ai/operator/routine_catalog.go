/*
|--------------------------------------------------------------------------
| A Routine's Catalog Reads
|--------------------------------------------------------------------------
|
| A routine is offered its few tools by name, so it cannot be handed the
| whole API catalog the way a chat is. It is offered the catalog's reads
| that answer what its own tools do not, by the tool group each belongs
| to, under the names the MCP server gives them. Each call is made as
| the routine's narrowed copy of its owner, so it sees only the
| routine's inboxes and holds only its permissions, and a run that made
| one counts as having read what a customer wrote.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"strings"
)

/*
routineReads are the catalog reads a routine is offered, each with the
routine tool group that brings it.
*/
func routineReads() map[string]string {
	return map[string]string{
		"dimmi_list_help_centers":             RoutineGroupKnowledge,
		"dimmi_list_help_center_collections":  RoutineGroupKnowledge,
		"dimmi_list_article_versions":         RoutineGroupKnowledge,
		"dimmi_get_article_feedback":          RoutineGroupKnowledge,
		"dimmi_list_procedure_versions":       RoutineGroupKnowledge,
		"dimmi_list_inboxes":                  RoutineGroupConversations,
		"dimmi_list_ai_agents":                RoutineGroupKnowledge,
		"dimmi_get_ai_agent_by_id":            RoutineGroupKnowledge,
		"dimmi_list_ai_agent_simulations":     RoutineGroupKnowledge,
		"dimmi_get_ai_agent_simulation_by_id": RoutineGroupKnowledge,
		"dimmi_list_ai_agent_simulation_runs": RoutineGroupInsights,
		"dimmi_get_help_center_analytics":     RoutineGroupInsights,
		"dimmi_get_procedure_metrics":         RoutineGroupInsights,
	}
}

/*
catalogTools are the catalog reads offered to a routine's run, none to
any other run or when the engine has no catalog.
*/
func (e *Engine) catalogTools(s *session) []tool {
	if s == nil || s.routine == nil || e.Catalog == nil {
		return nil
	}
	reads := routineReads()
	var out []tool
	for _, c := range e.Catalog.Tools() {
		group, ok := reads[c.Spec.Name]
		if !ok {
			continue
		}
		out = append(out, tool{spec: c.Spec, group: group, handle: e.catalogRead(c.Spec.Name)})
	}
	return out
}

/*
catalogRead returns the handler that makes one catalog read as the
run's teammate.
*/
func (e *Engine) catalogRead(name string) handler {
	return func(ctx context.Context, s *session, args json.RawMessage) (result, error) {
		out, err := e.Catalog.Call(ctx, s.m, CatalogCall{Name: name, Args: args})
		if err != nil {
			return result{}, err
		}
		s.tainted = true
		summary := "Read " + strings.ReplaceAll(strings.TrimPrefix(name, "dimmi_"), "_", " ")
		return result{text: out, step: Step{Tool: name, Summary: summary}}, nil
	}
}
