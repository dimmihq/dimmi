package ai_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

func TestAiTraceRetention(t *testing.T) {
	t.Parallel()

	t.Run("drops old payloads, keeps their steps and deletes runs past their time", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		now := d.TC.Clock.Now()
		days := func(n int) time.Time { return now.Add(-time.Duration(n) * 24 * time.Hour) }
		fresh := seedRun(t, d, agent, run{outcome: "answered", model: "m", at: days(1), payload: true})
		aged := seedRun(t, d, agent, run{outcome: "answered", model: "m", at: days(40), payload: true})
		tried := seedRun(t, d, agent, run{kind: "playground", outcome: "answered", model: "m", at: days(40)})
		ancient := seedRun(t, d, agent, run{outcome: "answered", model: "m", at: days(500), payload: true})

		job := &jobs.PruneAiTraces{
			DB: d.TC.App.DB, Clock: d.TC.Clock, Retention: d.TC.App.Retention,
			PayloadRetentionDays: 30, PlaygroundRetentionDays: 30, RunsRetentionDays: 400,
		}
		require.NoError(t, job.Handle(t.Context(), nil))

		runs := d.TC.Table("ai_runs")
		runs.AssertCount(1, testkit.Row{"id": fresh})
		runs.AssertCount(1, testkit.Row{"id": aged})
		runs.AssertCount(0, testkit.Row{"id": tried})
		runs.AssertCount(0, testkit.Row{"id": ancient})

		steps := d.TC.Table("ai_run_steps")
		steps.AssertCount(1, testkit.Row{"run_id": aged, "request": nil, "response": nil, "cost_micros": 0})
		steps.AssertCount(0, testkit.Row{"run_id": fresh, "request": nil})
		steps.AssertCount(0, testkit.Row{"run_id": ancient})
	})

	t.Run("keeps everything when retention is off", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		old := seedRun(t, d, agent, run{
			outcome: "answered", model: "m", at: d.TC.Clock.Now().AddDate(-3, 0, 0), payload: true,
		})

		job := &jobs.PruneAiTraces{DB: d.TC.App.DB, Clock: d.TC.Clock, Retention: d.TC.App.Retention}
		require.NoError(t, job.Handle(t.Context(), nil))

		d.TC.Table("ai_run_steps").AssertCount(0, testkit.Row{"run_id": old, "request": nil})
	})
}

func TestAiInboxFilter(t *testing.T) {
	t.Parallel()

	t.Run("finds the conversations the agent handed over or is answering", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		handed, handedID := conversationOf(t, d, "jane")
		active, activeID := conversationOf(t, d, "joe")
		conversationOf(t, d, "ann")
		_, err := d.TC.Operator().Exec(t.Context(), `
			UPDATE conversations SET ai_state = CASE WHEN id = $1 THEN 'handed_off' ELSE 'active' END,
				ai_reply_count = 1
			WHERE id IN ($1, $2)`, handedID, activeID)
		require.NoError(t, err)

		res := d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[status]=any&filter[ai]=handed_off").
			AssertOK()
		res.AssertJSONPath("data.#", 1)
		res.AssertJSONPath("data.0.id", handed)
		res.AssertJSONPath("data.0.ai_state", "handed_off")

		res = d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[status]=any&filter[ai]=active").AssertOK()
		res.AssertJSONPath("data.#", 1)
		res.AssertJSONPath("data.0.id", active)
		res.AssertJSONPath("data.0.ai_state", "active")

		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[ai]=sleeping").AssertStatus(400)
	})
}
