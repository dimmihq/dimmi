package workflows_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
splitRuns is the part of a workflow's runs the split tests read.
*/
type splitRuns struct {
	Data []struct {
		ConversationID string `json:"conversation_id"`
		Steps          []struct {
			StepID  string `json:"step_id"`
			Outcome string `json:"outcome"`
			PathID  string `json:"path_id"`
			Detail  struct {
				Variant string `json:"variant"`
			} `json:"detail"`
		} `json:"steps"`
	} `json:"data"`
}

/*
splitSpec is a workflow that splits every conversation between a high
and a low priority, half and half, with the winner when one is given.
*/
func splitSpec(trigger, winner string) testkit.JSON {
	return weightedSplit(trigger, winner, 50, 50)
}

/*
weightedSplit is splitSpec with the shares given.
*/
func weightedSplit(trigger, winner string, high, low int) testkit.JSON {
	settings := testkit.JSON{"variants": []testkit.JSON{
		{"id": "p_varnta", "label": "High", "weight": high},
		{"id": "p_varntb", "label": "Low", "weight": low},
	}}
	if winner != "" {
		settings["winner"] = winner
	}

	return testkit.JSON{
		"schema": 1, "trigger": trigger, "goal": testkit.JSON{"kind": "resolved", "window": "3d"},
		"steps": []testkit.JSON{step("s_split001", "split", settings,
			path("p_varnta", "", step("s_high0001", "priority", testkit.JSON{"value": "high"})),
			path("p_varntb", "", step("s_low00001", "priority", testkit.JSON{"value": "low"})),
		)},
	}
}

/*
splitTaken returns, by conversation id, the variant each run took at
the split, failing when a run took another or kept none.
*/
func splitTaken(t *testing.T, d tests.Desk, workflow string) map[string][]string {
	t.Helper()

	var got splitRuns
	d.Owner.GetJSON("/api/v1/workflows/" + workflow + "/runs?limit=50").AssertOK().Decode(&got)

	out := map[string][]string{}
	for _, r := range got.Data {
		for _, s := range r.Steps {
			if s.StepID != "s_split001" {
				continue
			}

			require.NotEmpty(t, s.PathID)
			assert.Equal(t, s.PathID, s.Detail.Variant, "the trace keeps the variant")
			out[r.ConversationID] = append(out[r.ConversationID], s.PathID+":"+s.Outcome)
		}
	}

	return out
}

/*
assertVariants checks the metrics count each variant's runs, and the
goal only for the variant of the conversation that was closed.
*/
func assertVariants(t *testing.T, res *testkit.Response, counts map[string]int, closed string) {
	t.Helper()

	for key, n := range counts {
		variant, _, _ := strings.Cut(key, ":")
		goal := 0
		if key == closed {
			goal = 1
		}

		res.AssertJSONPath("data.steps.s_split001.variants."+variant+".reached", float64(n)).
			AssertJSONPath("data.steps.s_split001.variants."+variant+".goal", float64(goal)).
			AssertJSONPath("data.steps.s_split001.paths."+variant, float64(n))
	}

	res.AssertJSONMissing("data.steps.s_high0001.variants")
}

func TestWorkflowSplit(t *testing.T) {
	t.Parallel()

	t.Run("gives each conversation one variant and keeps it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := published(t, d, "Priority test", splitSpec("message.customer", ""))

		conv := conversation(t, d, "ada", "Hello")
		for _, body := range []string{"Anyone there?", "Still there?"} {
			d.TC.Clock.Advance(time.Minute)
			d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": body}).AssertCreated()
			d.TC.Drain()
		}

		taken := splitTaken(t, d, id)
		require.Len(t, taken, 1)
		for _, variants := range taken {
			require.GreaterOrEqual(t, len(variants), 2)
			assert.Len(t, slices.Compact(variants), 1, "every run of a conversation takes its variant")
		}
	})

	t.Run("spreads conversations over the variants and compares their goal", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := published(t, d, "Priority test", splitSpec("conversation.first_message", ""))

		convs := make([]string, 0, 20)
		for i := range 20 {
			convs = append(convs, conversation(t, d, "c"+strconv.Itoa(i), "Hello"))
		}

		d.TC.Clock.Advance(time.Hour)
		d.Owner.PatchJSON(convs[0], testkit.JSON{"status": "closed"}).AssertOK()
		d.TC.Drain()
		refreshFacts(t, d)

		taken := splitTaken(t, d, id)
		counts := map[string]int{}
		for _, variants := range taken {
			require.Len(t, variants, 1)
			counts[variants[0]]++
		}
		assert.Positive(t, counts["p_varnta:variant"], "some take the first variant")
		assert.Positive(t, counts["p_varntb:variant"], "some take the second variant")

		closed := taken[convs[0][len("/api/v1/conversations/"):]][0]
		priority := map[string]string{"p_varnta:variant": "high", "p_varntb:variant": "low"}[closed]
		d.Owner.GetJSON(convs[0]).AssertOK().AssertJSONPath("data.priority", priority)

		assertVariants(t, d.Owner.GetJSON("/api/v1/workflows/"+id+"/metrics").AssertOK(), counts, closed)
	})

	t.Run("sends everyone down the winner once it is published", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := published(t, d, "Priority test", splitSpec("conversation.first_message", ""))

		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{
			"spec": splitSpec("conversation.first_message", "p_varntb"),
		}).AssertOK()
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()

		for i := range 6 {
			conversation(t, d, "w"+strconv.Itoa(i), "Hello")
		}

		taken := splitTaken(t, d, id)
		require.Len(t, taken, 6)
		for _, variants := range taken {
			assert.Equal(t, []string{"p_varntb:winner"}, variants)
		}
	})

	t.Run("refuses to publish shares that miss 100", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		spec := weightedSplit("conversation.first_message", "", 50, 20)
		id := create(t, d, testkit.JSON{"name": "Priority test", "spec": spec})

		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertStatus(422)
	})
}
