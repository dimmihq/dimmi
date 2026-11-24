package stowline_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

func TestSimulations(t *testing.T) {
	t.Parallel()

	ids := map[string]string{}
	for _, name := range []string{"get_shipment_status", "get_account_overview"} {
		ids[name] = "aac_" + name
	}
	names := map[string]bool{}
	sims := stowline.Simulations()
	require.Len(t, sims, 8)

	for _, sim := range sims {
		assert.False(t, names[sim.Name], "%s is named twice", sim.Name)
		names[sim.Name] = true
		assert.NotEmpty(t, sim.Messages, sim.Name)
		assert.NotEmpty(t, sim.Scenario, "%s says who its customer is, so no model writes it", sim.Name)
		assert.LessOrEqual(t, len([]rune(sim.Scenario)), 1000, sim.Name)
		assert.NotEmpty(t, sim.Persona.ExternalID, "%s has a signed-in customer Sky can look up", sim.Name)
		assert.Empty(t, procedure.CheckAssertions(sim.Checks), sim.Name)

		in := sim.Input(ids)
		require.NotNil(t, in.Scenario)
		assert.Len(t, in.Messages, len(sim.Messages))
		for name := range sim.Answers {
			assert.Contains(t, in.Mocks, ids[name], "%s answers %s under its id", sim.Name, name)
		}
	}
}

func TestSalesSimulations(t *testing.T) {
	t.Parallel()

	names := map[string]bool{}
	sims := stowline.SalesSimulations()
	require.Len(t, sims, 11)

	for _, sim := range sims {
		assert.False(t, names[sim.Name], "%s is named twice", sim.Name)
		names[sim.Name] = true
		assert.Len(t, sim.Messages, 1, sim.Name)
		assert.NotEmpty(t, sim.Scenario, "%s says who its buyer is, so no model writes it", sim.Name)
		assert.LessOrEqual(t, len([]rune(sim.Scenario)), 1000, sim.Name)
		assert.Empty(t, sim.Persona.ExternalID, "%s has a buyer who has not signed in", sim.Name)
		assert.Empty(t, sim.Answers, "%s needs no action answered, Rio has none of the workspace's", sim.Name)
		assert.Empty(t, procedure.CheckAssertions(sim.Checks), sim.Name)
	}
}
