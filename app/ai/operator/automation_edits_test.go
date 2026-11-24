package operator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

func edits[T any](t *testing.T, raw string) []T {
	t.Helper()
	var out []T
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

func buttonsSpec() workflowspec.Spec {
	return workflowspec.Spec{Schema: 1, Trigger: workflowspec.TriggerFirstMessage, Steps: []workflowspec.Step{
		{ID: "s_ask000001", Kind: "buttons", Settings: json.RawMessage(`{"text":"Pick"}`), Paths: []workflowspec.Path{
			{ID: "p_sales0001", Label: "Sales", Steps: []workflowspec.Step{{ID: "s_tag000001", Kind: "tag"}}},
			{ID: "p_help00001", Label: "Help", Steps: []workflowspec.Step{}},
		}},
		{ID: "s_end000001", Kind: "close"},
	}}
}

func TestWorkflowEdits(t *testing.T) {
	t.Parallel()

	t.Run("adds after a step, into a path and first, without touching what was read", func(t *testing.T) {
		t.Parallel()
		base := buttonsSpec()
		out, err := applyWorkflowEdits(base, edits[workflowEdit](t, `[
			{"action":"add_step","after_step_id":"s_tag000001","step":{"kind":"note","settings":"{\"text\":\"hi\"}"}},
			{"action":"add_step","into_path_id":"p_help00001","step":{"kind":"dimmi"}},
			{"action":"add_step","at_start":true,"step":{"kind":"message"}}
		]`))
		require.NoError(t, err)
		assert.Equal(t, "message", out.Steps[0].Kind)
		sales := out.Steps[1].Paths[0].Steps
		assert.Equal(t, []string{"tag", "note"}, []string{sales[0].Kind, sales[1].Kind})
		assert.JSONEq(t, `{"text":"hi"}`, string(sales[1].Settings))
		assert.Equal(t, "dimmi", out.Steps[1].Paths[1].Steps[0].Kind)
		assert.Len(t, base.Steps, 2)
		assert.Len(t, base.Steps[0].Paths[0].Steps, 1)
	})

	t.Run("changes, moves and removes steps and paths, and sets the trigger's settings", func(t *testing.T) {
		t.Parallel()
		out, err := applyWorkflowEdits(buttonsSpec(), edits[workflowEdit](t, `[
			{"action":"change_step","step_id":"s_ask000001","settings":{"text":"Which one?"}},
			{"action":"move_step","step_id":"s_end000001","into_path_id":"p_help00001"},
			{"action":"change_path","step_id":"s_ask000001","path_id":"p_sales0001","label":"Buy"},
			{"action":"add_path","step_id":"s_ask000001","path":{"label":"Other"}},
			{"action":"remove_step","step_id":"s_tag000001"},
			{"action":"set_trigger","channels":["email"],"exclusive":true}
		]`))
		require.NoError(t, err)
		require.Len(t, out.Steps, 1)
		ask := out.Steps[0]
		assert.JSONEq(t, `{"text":"Which one?"}`, string(ask.Settings))
		assert.Equal(t, "Buy", ask.Paths[0].Label)
		assert.Empty(t, ask.Paths[0].Steps)
		assert.Equal(t, "close", ask.Paths[1].Steps[0].Kind)
		assert.Equal(t, "Other", ask.Paths[2].Label)
		assert.Equal(t, []string{"email"}, out.Channels)
		assert.True(t, out.Exclusive)
	})

	t.Run("says which edit names something the workflow does not have", func(t *testing.T) {
		t.Parallel()
		_, err := applyWorkflowEdits(buttonsSpec(), edits[workflowEdit](t, `[
			{"action":"remove_step","step_id":"s_end000001"},
			{"action":"change_path","step_id":"s_ask000001","path_id":"p_nope"}
		]`))
		require.ErrorIs(t, err, errBadArguments)
		assert.Contains(t, err.Error(), `edit 2 (change_path): step s_ask000001 has no path "p_nope"; its paths are `+
			`p_sales0001 (Sales), p_help00001 (Help)`)
	})

	t.Run("outlines every step and path with its id", func(t *testing.T) {
		t.Parallel()
		outline := workflowOutline(buttonsSpec())
		assert.Contains(t, outline, "  - step s_ask000001 · buttons · settings {\"text\":\"Pick\"}\n")
		assert.Contains(t, outline, "    path p_sales0001 · label \"Sales\"\n      - step s_tag000001 · tag\n")
		assert.Contains(t, outline, "    path p_help00001 · label \"Help\"\n      (no steps: the run ends here)\n")
	})
}

func seriesSpec() series.Spec {
	spec := series.NewSpec()
	spec.Nodes = append(spec.Nodes,
		series.Node{ID: "hello", Kind: series.KindEmail, Settings: json.RawMessage(`{"subject":"Hi","body":"Hello"}`)},
		series.Node{ID: "wait", Kind: series.KindWait, Settings: json.RawMessage(
			`{"mode":"duration","amount":2,"unit":"days"}`,
		)},
	)
	spec.Edges = []series.Edge{{From: "entry", Path: "next", To: "hello"}, {From: "hello", Path: "next", To: "wait"}}
	return spec
}

func TestSeriesEdits(t *testing.T) {
	t.Parallel()

	t.Run("puts a new step on a path, between the two steps it joined", func(t *testing.T) {
		t.Parallel()
		out, err := applySeriesEdits(seriesSpec(), edits[seriesEdit](t, `[
			{"action":"add_step","from_step_id":"hello","step":{"kind":"tag","settings":{"tag_ids":["tag_1"]}}}
		]`))
		require.NoError(t, err)
		assert.Equal(t, "tag-1", out.Nodes[3].ID)
		assert.ElementsMatch(t, []series.Edge{
			{From: "entry", Path: "next", To: "hello"},
			{From: "hello", Path: "next", To: "tag-1"},
			{From: "tag-1", Path: "next", To: "wait"},
		}, out.Edges)
	})

	t.Run("removes a step, joining the step before to the step after", func(t *testing.T) {
		t.Parallel()
		out, err := applySeriesEdits(seriesSpec(), edits[seriesEdit](t, `[{"action":"remove_step","step_id":"hello"}]`))
		require.NoError(t, err)
		assert.Len(t, out.Nodes, 2)
		assert.Equal(t, []series.Edge{{From: "entry", Path: "next", To: "wait"}}, out.Edges)
	})

	t.Run("joins and cuts paths and sets the goal", func(t *testing.T) {
		t.Parallel()
		out, err := applySeriesEdits(seriesSpec(), edits[seriesEdit](t, `[
			{"action":"disconnect","from_step_id":"hello"},
			{"action":"connect","from_step_id":"hello","path":"failed","to_step_id":"wait"},
			{"action":"set_goal","goal":{"event":"paid","days":14}}
		]`))
		require.NoError(t, err)
		assert.Contains(t, out.Edges, series.Edge{From: "hello", Path: "failed", To: "wait"})
		assert.NotContains(t, out.Edges, series.Edge{From: "hello", Path: "next", To: "wait"})
		require.NotNil(t, out.Goal)
		assert.Equal(t, 14, out.Goal.Days)
	})

	t.Run("refuses a path the step does not have, and keeps the entry", func(t *testing.T) {
		t.Parallel()
		_, err := applySeriesEdits(seriesSpec(), edits[seriesEdit](t, `[
			{"action":"add_step","from_step_id":"wait","path":"matched","step":{"kind":"exit"}}
		]`))
		assert.ErrorContains(t, err, `step wait has no path "matched"; its paths are next, no_date`)
		_, err = applySeriesEdits(seriesSpec(), edits[seriesEdit](t, `[{"action":"remove_step","step_id":"entry"}]`))
		assert.ErrorContains(t, err, "the entry step stays")
	})

	t.Run("outlines where each path leads", func(t *testing.T) {
		t.Parallel()
		outline := seriesOutline(seriesSpec())
		assert.Contains(t, outline, "- step hello · email · settings {\"subject\":\"Hi\",\"body\":\"Hello\"}\n")
		assert.Contains(t, outline, "    path next → wait\n    path failed → (nothing: the journey ends)\n")
	})
}

func TestAutomationReference(t *testing.T) {
	t.Parallel()

	t.Run("writes each kind's settings and paths from the checker's registries", func(t *testing.T) {
		t.Parallel()
		ref := workflowReference(workflowspec.TriggerTeammateMessage)
		assert.Contains(t, ref, "- message.teammate: Teammate sends a reply.")
		assert.Contains(t, ref, "- buttons (not allowed under this trigger): settings {allow_typing?: boolean, "+
			"retry_text?: string, text: string, timeout?: string}. Paths you add: 2 to 6. Fixed paths, by outcome: "+
			"typed, timeout.")
		assert.Contains(t, ref, "- wait: settings {count?: string, for?: string, interrupt_on?: [string], "+
			"schedule_id?: string, until?: {kind: string, schedule_id?: string, time?: string}}.")
		assert.Contains(t, ref, "- branch. Paths you add: 1 to 10. Fixed paths, by outcome: else (always has else).")
		assert.Contains(t, ref, "- split: settings {variants: [{id: string, label: string, weight: number}], "+
			"winner?: string}. Paths you add: 2 to 4.")
		assert.Contains(t, ref, "conversation.priority (list: none, low, medium, high, urgent)")
		assert.NotContains(t, ref, "- conversation.opener:")
	})

	t.Run("writes series kinds with their paths", func(t *testing.T) {
		t.Parallel()
		ref := seriesReference()
		assert.Contains(t, ref, "- email: settings {body: string, preview?: string, recipients?: json, "+
			"subject?: string, track_clicks?: boolean}. Paths: next, failed.")
		assert.Contains(t, ref, "Paths: matched, timed_out.")
		assert.Contains(t, ref, "Goal: {control_percent?: number, days: number, event?: string, rules?: json}")
		assert.Contains(t, ref, "- exit. No paths: the journey ends here.")
	})
}
