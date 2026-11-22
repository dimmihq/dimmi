package ai_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

/*
published records what the pipeline broadcasts.
*/
type published struct {
	mu   sync.Mutex
	sent []fbroadcasting.Message
	to   [][]string
}

func (p *published) Publish(_ context.Context, channels []string, m fbroadcasting.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, m)
	p.to = append(p.to, channels)
	return nil
}

/*
moments lists the moments of a simulation's run that were broadcast,
in order.
*/
func (p *published) moments() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, m := range p.sent {
		data, _ := m.Data.(map[string]any)
		if moment, ok := data["moment"].(string); ok && m.Event == pipeline.EventSimulationRun {
			out = append(out, moment)
		}
	}
	return out
}

/*
TestSimulationRunIsWatched covers a saved simulation's run as the
teammate who started it sees it happen.
*/
func TestSimulationRunIsWatched(t *testing.T) {
	t.Parallel()

	saved := func(t *testing.T) (*harness, *published, string) {
		t.Helper()
		h := newHarness(t, nil)
		out := &published{}
		h.d.TC.App.AI.Publisher = out
		id := h.liveProcedure("Cancel", testkit.JSON{
			"description": "The customer wants to cancel.",
			"steps":       []testkit.JSON{{"id": "why", "kind": "instruction", "text": "Say the team will cancel it."}},
			"handover":    []string{},
		})
		base := "/api/v1/procedures/" + id + "/test-cases"
		h.d.Owner.PostJSON(base, testkit.JSON{
			"name": "Cancel", "messages": []testkit.JSON{{"text": "Cancel my plan"}, {"text": "Thanks"}},
			"assertions": []testkit.JSON{{"type": "handed_off", "handed_off": false}},
		}).AssertCreated()
		h.llm.answer.Script(reply("The team will cancel your plan."))

		return h, out, base + "/run"
	}

	t.Run("tells the teammate each turn as it happens, then the verdict", func(t *testing.T) {
		t.Parallel()
		h, out, run := saved(t)

		h.d.Owner.PostJSON(run, testkit.JSON{"stream_id": "stream-1"}).AssertOK().
			AssertJSONPath("data.0.last_result.passed", true)

		assert.Equal(t, []string{
			"started", "turn_started", "turn_finished", "turn_started", "turn_finished", "judging", "finished",
		}, out.moments())
		require.NotEmpty(t, out.sent)
		for i, m := range out.sent {
			assert.True(t, m.Ephemeral, "a run's events are never stored")
			require.Len(t, out.to[i], 1, "only the teammate who started the run is told")
			data, _ := m.Data.(map[string]any)
			id, _ := data["stream_id"].(string)
			assert.Contains(t, id, "stream-1")
		}
	})

	t.Run("tells nobody when the run was not given a stream", func(t *testing.T) {
		t.Parallel()
		h, out, run := saved(t)

		h.d.Owner.PostJSON(run, nil).AssertOK()

		assert.Empty(t, out.moments())
	})
}
