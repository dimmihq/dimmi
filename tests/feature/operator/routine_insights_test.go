package operator_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestOperatorRoutineInsights(t *testing.T) {
	t.Parallel()

	t.Run("counts a routine's suggestions and declines the stale ones", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Refund please")
		number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		routine := k.routine(testkit.JSON{"name": "Tagger", "prompt": "Tag refunds."})
		k.llm.Script(tools(call("propose_changes", tagging(t, number, ""))), reply("Suggested a tag."))
		k.runRoutine(routine, false)

		k.d.Owner.GetJSON("/api/v1/operator/routines/"+routine).AssertOK().
			AssertJSONPath("data.suggestions.pending", float64(1)).
			AssertJSONPath("data.suggestions.declined", float64(0))
		k.d.Owner.GetJSON("/api/v1/operator/routines").AssertOK().
			AssertJSONPath("data.0.suggestions.pending", float64(1))

		id, err := k.d.TC.App.IDs.Decode(string(models.PrefixOperatorRoutine), routine)
		require.NoError(t, err)
		past := k.d.TC.App.Clock.Now().Add(-time.Hour)
		_, err = k.d.TC.Operator().Exec(k.t.Context(),
			`UPDATE operator_changesets SET expires_at = $2 WHERE routine_id = $1`, id, past)
		require.NoError(t, err)

		n, err := k.d.TC.App.Actions.OperatorAgent.Engine.ExpireSuggestions(k.t.Context())
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(1))

		k.d.Owner.GetJSON("/api/v1/operator/changesets?filter[routine]="+routine).AssertOK().
			AssertJSONPath("data.0.status", "declined")
		k.d.Owner.GetJSON("/api/v1/operator/routines/"+routine).AssertOK().
			AssertJSONPath("data.suggestions.pending", float64(0)).
			AssertJSONPath("data.suggestions.declined", float64(1))
	})

	t.Run("suggests again what went stale", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Refund please")
		number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		routine := k.routine(testkit.JSON{"name": "Tagger", "prompt": "Tag refunds."})
		k.llm.Script(tools(call("propose_changes", tagging(t, number, "tag:1"))), reply("One."))
		k.runRoutine(routine, false)
		id, err := k.d.TC.App.IDs.Decode(string(models.PrefixOperatorRoutine), routine)
		require.NoError(t, err)
		_, err = k.d.TC.Operator().Exec(k.t.Context(),
			`UPDATE operator_changesets SET expires_at = $2 WHERE routine_id = $1`, id,
			k.d.TC.App.Clock.Now().Add(-time.Hour))
		require.NoError(t, err)
		_, err = k.d.TC.App.Actions.OperatorAgent.Engine.ExpireSuggestions(k.t.Context())
		require.NoError(t, err)
		k.llm.Script(tools(call("propose_changes", tagging(t, number, "tag:1"))), reply("Again."))

		k.runRoutine(routine, false).AssertJSONPath("data.changesets_count", float64(1))
	})

	t.Run("leaves suggestions that are still fresh", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Refund please")
		number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		routine := k.routine(testkit.JSON{"name": "Tagger", "prompt": "Tag refunds."})
		k.llm.Script(tools(call("propose_changes", tagging(t, number, ""))), reply("Suggested a tag."))
		k.runRoutine(routine, false)

		_, err := k.d.TC.App.Actions.OperatorAgent.Engine.ExpireSuggestions(k.t.Context())
		require.NoError(t, err)

		k.d.Owner.GetJSON("/api/v1/operator/changesets?filter[routine]="+routine).AssertOK().
			AssertJSONPath("data.0.status", "pending")
	})
}
