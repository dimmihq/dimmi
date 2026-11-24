package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestAiAgentSimulationSpend(t *testing.T) {
	t.Parallel()

	t.Run("tallies simulations apart from the spend the cap reads", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		h.llm.answer.Script(reply("Growth is $29 a seat each month."), reply("Growth is $29 a seat each month."))
		id := h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
			"name":       "Quotes the price",
			"messages":   []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
		}).AssertCreated().String("data.id")

		for range 2 {
			h.d.Owner.PostJSON(path+"/simulations/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
		}

		var simulations, customerRuns int
		var capped, simulationCost int64
		var cappedAt *string
		require.NoError(t, h.d.TC.Operator().QueryRow(t.Context(),
			`SELECT simulation_runs_count, simulation_cost_micros, runs_count, cost_micros, capped_at::text
			 FROM ai_spend_periods WHERE workspace_id = $1`, h.d.TC.Workspace.Workspace.ID,
		).Scan(&simulations, &simulationCost, &customerRuns, &capped, &cappedAt))

		assert.Equal(t, 2, simulations, "each finished simulation is counted once")
		assert.GreaterOrEqual(t, simulationCost, int64(0))
		assert.Zero(t, customerRuns, "a simulation is no customer run")
		assert.Zero(t, capped, "what simulations cost is no part of the spend the cap reads")
		assert.Nil(t, cappedAt, "simulating never caps the month")
	})
}
