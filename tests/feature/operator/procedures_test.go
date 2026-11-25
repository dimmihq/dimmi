package operator_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
procedureFixture publishes a procedure and seeds one run of it that the
procedure's guardrail handed over, returning the procedure's and the
conversation's number.
*/
func (k *desk) procedureFixture() (string, string) {
	k.t.Helper()
	proc := k.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Refunds"}).AssertCreated().String("data.id")
	k.d.Owner.PatchJSON("/api/v1/procedures/"+proc, testkit.JSON{"definition": testkit.JSON{
		"description": "Refund requests.",
		"steps":       []testkit.JSON{{"id": "s1", "kind": "instruction", "text": "Check the charge."}},
		"handover":    []string{},
	}}).AssertOK()
	k.d.Owner.PostJSON("/api/v1/procedures/"+proc+"/publications", nil).AssertOK()
	conv := k.conversation("Refund my order, my email is ana@example.com")
	number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")

	var run string
	err := k.d.TC.Operator().QueryRow(k.t.Context(), `
		INSERT INTO ai_runs (workspace_id, agent_id, kind, conversation_id, status, outcome, handoff_reason, flags,
			procedure_trace, created_at)
		SELECT c.workspace_id, a.id, 'conversation', c.id, 'completed', 'handoff', 'procedure_guard',
			ARRAY['procedure:' || p.id::text],
			jsonb_build_object('procedure_id', $1::text, 'guard_events',
				jsonb_build_array(jsonb_build_object(
					'type', 'compliance_failed_handoff', 'missing_tool', 'get_order'))),
			$3::timestamptz
		FROM conversations c
		JOIN ai_agents a ON a.workspace_id = c.workspace_id
		JOIN procedures p ON p.workspace_id = c.workspace_id AND p.name = 'Refunds'
		WHERE c.number = $2::bigint
		RETURNING id::text`, proc, number, k.d.TC.App.Clock.Now()).Scan(&run)
	require.NoError(k.t, err)

	return proc, number
}

func TestOperatorProcedureQA(t *testing.T) {
	t.Parallel()

	t.Run("finds the runs that went wrong and reads their traces", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		proc, number := k.procedureFixture()
		thread := k.thread("")
		k.llm.Script(tools(call("list_procedure_anomalies", map[string]any{"procedure_id": proc})), reply("Found one."))

		k.ask(thread, "How is the refunds procedure doing?").AssertOK()

		sent := k.llm.Sent()
		require.Len(t, sent, 2)
		result := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, result, "conversation #"+number)
		assert.Contains(t, result, "compliance_failed_handoff")
		assert.Contains(t, result, "handed_off")
		run := result[strings.Index(result, "air_") : strings.Index(result, "air_")+30]

		k.llm.Script(
			tools(call("read_procedure_trace", map[string]any{"run_id": strings.TrimSpace(run)})),
			reply("Read."),
		)
		k.ask(thread, "Read it").AssertOK()
		trace := k.llm.Sent()[3].Messages[len(k.llm.Sent()[3].Messages)-1].Parts[0].Text
		assert.Contains(t, trace, `"missing_tool": "get_order"`)
		assert.Contains(t, trace, "CUSTOMER: Refund my order")
		assert.NotContains(t, trace, "ana@example.com", "the transcript is redacted")
	})

	t.Run("proposes a regression test that applies and reverts", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		proc, number := k.procedureFixture()
		thread := k.thread("")
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Guard the lookup", "reason": "Conversation #" + number + " skipped the lookup.",
			"operations": []map[string]any{{
				"kind": "procedure.create_test_case", "procedure": proc,
				"test_case": map[string]any{
					"name": "Looks the order up first", "source_conversation": atoi(t, number),
					"messages":   []map[string]any{{"text": "Refund my order 1042"}},
					"assertions": []map[string]any{{"type": "action_called", "action": "get_order"}},
				},
			}},
		})), reply("Proposed."))

		res := k.ask(thread, "Add a test").AssertOK()
		id := res.String("data.changesets.0.id")
		assert.Equal(t, "Add the test “Looks the order up first” to the procedure “Refunds”",
			res.String("data.changesets.0.operations.0.label"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		tests := k.d.Owner.GetJSON("/api/v1/procedures/" + proc + "/test-cases").AssertOK()
		tests.AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.name", "Looks the order up first")
		assert.NotEmpty(t, tests.String("data.0.source_conversation_id"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "reverted")
		k.d.Owner.GetJSON("/api/v1/procedures/"+proc+"/test-cases").AssertOK().AssertJSONPath("data.#", float64(0))
	})
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err)
	return n
}
