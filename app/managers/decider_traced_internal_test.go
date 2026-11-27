package managers

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
)

func TestTracedDecider(t *testing.T) {
	t.Parallel()

	question := contracts.DecisionRequest{
		State:     "Ana asks where her order is",
		Questions: map[string]contracts.DecisionQuestion{"wants_person": {Type: contracts.DecisionNoul}},
	}

	t.Run("runs a decision outside any agent inside a background run", func(t *testing.T) {
		t.Parallel()
		_, tel, sent := tracked(t)
		fake, err := decisionfake.New("testing")
		require.NoError(t, err)
		fake.Script(decisionfake.Step{Usage: contracts.DecisionUsage{InputTokens: 40}})

		_, err = TracedDecider(tel, fake, "clef").Decide(t.Context(), question)
		require.NoError(t, err)

		txs := sent()
		require.Len(t, txs, 1)
		root := txs[0].Contexts.Trace
		assert.Equal(t, "background: decision", root.Data["gen_ai.agent.name"])
		require.Len(t, txs[0].Spans, 1)
		chat := txs[0].Spans[0]
		assert.Equal(t, "gen_ai.chat", chat.Op)
		assert.Equal(t, "clef", chat.Data["gen_ai.system"])
		assert.Equal(t, "wants_person", chat.Data["dimmi.decision.questions"])
		assert.InDelta(t, 40, chat.Data["gen_ai.usage.input_tokens"], 0)
	})

	t.Run("marks a failed decision failed, and keeps the state out", func(t *testing.T) {
		t.Parallel()
		_, tel, sent := tracked(t)
		fake, err := decisionfake.New("testing")
		require.NoError(t, err)
		fake.Script(decisionfake.Step{Err: errors.New("down")})

		ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		_, err = TracedDecider(tel, fake, "clef").Decide(ctx, question)
		require.Error(t, err)
		run.End()

		txs := sent()
		require.Len(t, txs, 1)
		require.Len(t, txs[0].Spans, 1, "inside a run, no background run of its own")
		assert.Equal(t, "internal_error", txs[0].Spans[0].Status)
		for _, s := range append(txs[0].Spans, txs[0].Contexts.Trace) {
			for _, v := range s.Data {
				text, _ := v.(string)
				assert.False(t, strings.Contains(text, "Ana"), "the state stays out")
			}
		}
	})
}
