/*
|--------------------------------------------------------------------------
| AI Agents Side By Side
|--------------------------------------------------------------------------
|
| One row per AI agent: the conversations it took part in, how many it
| resolved and handed to the team, how many it handed to another agent,
| what customers rated it and what it cost per resolution. A
| conversation counts for the agent that held it last, so a transfer
| credits the agent that finished the work; what a run cost counts for
| the agent that ran it.
|
*/

package reports

import (
	"cmp"
	"context"
	"slices"

	"github.com/dimmihq/dimmi/app/models"
)

/*
agentColumns are the figures each agent's row shows.
*/
func agentColumns() []Column {
	return []Column{
		{Key: "involved", Label: "Conversations", Unit: UnitCount},
		{Key: "resolution", Label: "Resolved", Unit: UnitPercent},
		{Key: "handoff", Label: "Handed over", Unit: UnitPercent},
		{Key: "transfers", Label: "Transferred", Unit: UnitCount},
		{Key: "satisfaction", Label: "CSAT", Unit: UnitPercent},
	}
}

/*
agentRuns is how often one agent's runs transferred.
*/
type agentRuns struct {
	transfers int64
}

/*
agentsTable compares the workspace's AI agents.
*/
func (r reader) agentsTable(ctx context.Context) (Chart, error) {
	n, err := loadNames(ctx, r.tx, r.q.Membership.Workspace.ID)
	if err != nil {
		return Chart{}, err
	}

	started, err := r.startedFigures(ctx, groupAgent)
	if err != nil {
		return Chart{}, err
	}

	ratings, err := r.ratingFigures(ctx, groupAgent)
	if err != nil {
		return Chart{}, err
	}

	runs, err := r.agentRuns(ctx)
	if err != nil {
		return Chart{}, err
	}

	out := Chart{
		Key: "ai_agents", Title: "Conversations by AI agent", Kind: ChartTable, Axis: AxisCategory,
		Drill: "ai_involvement", Columns: agentColumns(),
	}
	for _, v := range started {
		if v.count(1) == 0 {
			continue
		}

		out.Rows = append(out.Rows, n.row(groupAgent, v.key, agentValues(v, keyed(ratings)[v.key], runs[v.key])))
	}

	slices.SortStableFunc(out.Rows, func(a, c Row) int { return cmp.Compare(first(c), first(a)) })

	return out, nil
}

/*
agentValues picks one agent's figures from its conversations, ratings
and runs.
*/
func agentValues(v, rated values, runs agentRuns) []*float64 {
	involved, handed, resolved := v.count(1), v.count(2), v.count(3)

	return []*float64{
		num(involved), ratio(resolved, involved), ratio(handed, involved), num(runs.transfers),
		ratio(rated.count(1), rated.count(0)),
	}
}

/*
keyed indexes grouped figures by their group.
*/
func keyed(rows []values) map[string]values {
	out := make(map[string]values, len(rows))
	for _, v := range rows {
		out[v.key] = v
	}

	return out
}

/*
agentRuns reads each agent's transfers in the range, keyed
like grouped figures.
*/
func (r reader) agentRuns(ctx context.Context) (map[string]agentRuns, error) {
	rows, err := models.New(r.tx).ReportAiAgentRuns(ctx, models.ReportAiAgentRunsParams{
		WorkspaceID: r.q.Membership.Workspace.ID, Since: r.q.Start, Until: r.q.End, Agents: r.q.Filters.Agents,
	})
	if err != nil {
		return nil, err
	}

	out := make(map[string]agentRuns, len(rows))
	for _, row := range rows {
		out[row.AgentID.String()] = agentRuns{transfers: row.Transfers}
	}

	return out, nil
}
