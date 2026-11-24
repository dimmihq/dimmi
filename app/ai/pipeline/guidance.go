/*
|--------------------------------------------------------------------------
| Agent Guidance
|--------------------------------------------------------------------------
|
| An agent's guidance names procedures, articles, actions, customer
| details, teams, tags and ticket types as chips, kept in the text as
| [[kind:ref]] the way a procedure step keeps them. The model reads each
| chip spelled out by name, with the tool it points at where there is
| one; a chip whose target is gone reads as something no longer there.
|
*/

package pipeline

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
guidanceArticles names the articles the agent's guidance cites.
*/
func guidanceArticles(ctx context.Context, q *models.Queries, agent models.AiAgent) (map[string]string, error) {
	var wanted []ids.UUID
	for _, m := range procedure.Markers(agent.Guidance) {
		if id, ok := decodePublic(models.PrefixArticle, m.Ref); ok && m.Kind == procedure.BindArticle {
			wanted = append(wanted, id)
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	rows, err := q.ListArticleTitles(ctx, models.ListArticleTitlesParams{WorkspaceID: agent.WorkspaceID, Ids: wanted})
	if err != nil {
		return nil, fmt.Errorf("list guidance articles: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[models.PublicID(models.PrefixArticle, r.ID)] = r.Title
	}
	return out, nil
}

/*
guidanceText is the agent's guidance with every chip spelled out.
*/
func guidanceText(st *run) string {
	text := st.env.Agent.Guidance
	markers := procedure.Markers(text)
	if len(markers) == 0 {
		return text
	}
	labels := procedureLabels(st.env)
	for public, title := range st.env.GuidanceArticles {
		labels[procedure.BindArticle+":"+public] = title
	}
	tools := actionTools(st)
	for _, m := range slices.Backward(markers) {
		text = text[:m.Start] + guidanceChip(m, labels, tools) + text[m.End:]
	}
	return text
}

/*
guidanceChip writes one chip of the guidance.
*/
func guidanceChip(m procedure.Marker, labels, tools map[string]string) string {
	if m.Kind == procedure.BindAction {
		if tool, ok := tools[m.Ref]; ok {
			return "the " + tool + " tool"
		}
		return "an action that is not available here"
	}
	if m.Kind == procedure.BindAttribute {
		return "the customer's " + m.Ref
	}
	label, ok := labels[m.Kind+":"+m.Ref]
	if !ok {
		return "something that no longer exists"
	}
	switch m.Kind {
	case procedure.BindArticle:
		return "the help article \"" + label + "\""
	case procedure.BindHandoff:
		return "the " + label + " team (hand over with the handoff tool)"
	case procedure.BindTag:
		return "the tag \"" + label + "\" (add it with triage_conversation)"
	case procedure.BindTicketType:
		return "a \"" + label + "\" ticket (open it with open_ticket)"
	case procedure.BindSubProcedure:
		return "the \"" + label + "\" procedure (start it with follow_procedure)"
	}
	return label
}
