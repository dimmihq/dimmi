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

func TestOperatorRoutineOneTarget(t *testing.T) {
	t.Parallel()

	t.Run("refuses a change request that touches two procedures", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		definition := testkit.JSON{
			"description": "Refund requests.",
			"steps":       []testkit.JSON{{"id": "s1", "kind": "instruction", "text": "Check the charge."}},
			"handover":    []string{},
		}
		var procs []string
		for _, name := range []string{"Refunds", "Returns"} {
			procs = append(procs, k.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": name}).
				AssertCreated().String("data.id"))
		}
		routine := k.routine(testkit.JSON{"name": "Tuner", "prompt": "Tune procedures."})
		ops := []map[string]any{}
		for _, p := range procs {
			ops = append(ops, map[string]any{"kind": "procedure.edit_draft", "procedure": p, "definition": definition})
		}
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Tune both", "reason": "Both need it.", "operations": ops,
		})), reply("Could not."))

		k.runRoutine(routine, false).AssertJSONPath("data.changesets_count", float64(0))

		sent := k.llm.Sent()
		result := sent[len(sent)-1].Messages[len(sent[len(sent)-1].Messages)-1].Parts[0].Text
		assert.Contains(t, result, "only one article or one procedure")
	})
}
