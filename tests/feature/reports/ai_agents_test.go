package reports_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
agentRow is one row of the AI agents table.
*/
type agentRow struct {
	Key    string     `json:"key"`
	Label  string     `json:"label"`
	Values []*float64 `json:"values"`
}

/*
agentsTable reads the AI agents table of the AI report.
*/
func agentsTable(t *testing.T, res *testkit.Response) []agentRow {
	t.Helper()

	var body struct {
		Data struct {
			Charts []struct {
				Key  string     `json:"key"`
				Rows []agentRow `json:"rows"`
			} `json:"charts"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, c := range body.Data.Charts {
		if c.Key == "ai_agents" {
			return c.Rows
		}
	}

	t.Fatalf("no ai_agents table in %s", res.Body)

	return nil
}

/*
exec runs SQL as the operator, scoped by the workspace as $1.
*/
func (d desk) exec(sql string) {
	d.t.Helper()
	_, err := d.TC.Operator().Exec(d.t.Context(), sql, d.TC.Workspace.Workspace.ID)
	require.NoError(d.t, err)
}

func TestReportAIAgents(t *testing.T) {
	t.Run("agents sit side by side and the agent filter keeps one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		sales := d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales"}).AssertCreated().
			String("data.agent.id")
		d.customer("ana")
		d.customer("bo")

		d.exec(`INSERT INTO messages (workspace_id, conversation_id, type, author_type, body)
			SELECT workspace_id, id, 'message', 'agent', 'Here is how' FROM conversations WHERE workspace_id = $1`)
		d.exec(`UPDATE conversations c SET ai_agent_id = a.id FROM ai_agents a
			WHERE c.workspace_id = $1 AND a.workspace_id = $1 AND a.name = 'Sales'
			  AND c.number = (SELECT min(number) FROM conversations WHERE workspace_id = $1)`)
		d.exec(`UPDATE conversations SET ai_agent_id = (SELECT id FROM ai_agents WHERE workspace_id = $1
			  AND name <> 'Sales' AND deleted_at IS NULL ORDER BY created_at LIMIT 1)
			WHERE workspace_id = $1 AND ai_agent_id IS NULL`)
		d.exec(`INSERT INTO ai_runs (workspace_id, agent_id, kind, status, outcome, cost_micros, created_at)
			SELECT $1, id, 'conversation', 'completed', 'transferred', 4000,
			  (SELECT max(created_at) FROM conversations WHERE workspace_id = $1) FROM ai_agents
			WHERE workspace_id = $1 AND name = 'Sales'`)
		d.exec(`SELECT dimmi_report_mark($1, id) FROM conversations WHERE workspace_id = $1`)
		d.refresh()

		rows := agentsTable(t, d.Owner.GetJSON("/api/v1/reports/ai").AssertOK())
		require.Len(t, rows, 2)
		byKey := map[string]agentRow{}
		for _, r := range rows {
			byKey[r.Key] = r
		}
		require.Contains(t, byKey, sales)
		assert.Equal(t, "Sales", byKey[sales].Label)
		assert.InDelta(t, 1, *byKey[sales].Values[0], 0)
		assert.InDelta(t, 1, *byKey[sales].Values[3], 0)

		only := agentsTable(t, d.Owner.GetJSON("/api/v1/reports/ai?filter[agent]="+sales).AssertOK())
		require.Len(t, only, 1)
		assert.Equal(t, sales, only[0].Key)
	})
}
