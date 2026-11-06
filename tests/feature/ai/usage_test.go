package ai_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestAiUsage(t *testing.T) {
	t.Parallel()

	t.Run("adds up the month from the tally and its runs", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		day := func(month time.Month, date, hour int) time.Time {
			return time.Date(2026, month, date, hour, 30, 0, 0, time.UTC)
		}
		seedRun(t, d, agent, run{outcome: "answered", cost: 1000, model: "claude-a", at: day(11, 1, 10)})
		seedRun(t, d, agent, run{outcome: "unknown", cost: 3000, model: "claude-b", at: day(11, 2, 7)})
		seedRun(t, d, agent, run{outcome: "handoff", cost: 500, model: "claude-a", at: day(11, 2, 8)})
		seedRun(t, d, agent, run{outcome: "answered", cost: 7000, model: "claude-a", at: day(10, 31, 23)})
		_, err := d.TC.Operator().Exec(t.Context(), `
			INSERT INTO ai_spend_periods (
				workspace_id, period_start, cost_micros, input_tokens, output_tokens,
				runs_count, answered_count, unknown_count, handoff_count
			) VALUES ($1, '2026-11-01', 4500, 300, 60, 3, 1, 1, 1)`, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 25}).AssertOK()

		res := d.Owner.GetJSON("/api/v1/ai/usage").AssertOK()

		res.AssertJSONPath("data.period_start", "2026-11-01")
		res.AssertJSONPath("data.cost_micros", 4500)
		res.AssertJSONPath("data.input_tokens", 300)
		res.AssertJSONPath("data.runs_count", 3)
		res.AssertJSONPath("data.answered_count", 1)
		res.AssertJSONPath("data.unknown_count", 1)
		res.AssertJSONPath("data.handoff_count", 1)
		res.AssertJSONPath("data.monthly_spend_cap_micros", 25_000_000)
		res.AssertJSONPath("data.daily", []testkit.JSON{
			{"day": "2026-11-01", "cost_micros": 1000, "runs_count": 1},
			{"day": "2026-11-02", "cost_micros": 3500, "runs_count": 2},
		})
		res.AssertJSONPath("data.by_model", []testkit.JSON{
			{"model": "claude-b", "cost_micros": 3000, "calls": 1},
			{"model": "claude-a", "cost_micros": 1500, "calls": 2},
		})
		res.AssertJSONPath("data.by_outcome", []testkit.JSON{
			{"outcome": "answered", "runs_count": 1},
			{"outcome": "handoff", "runs_count": 1},
			{"outcome": "unknown", "runs_count": 1},
		})

		past := d.Owner.GetJSON("/api/v1/ai/usage?period=2026-10").AssertOK()
		past.AssertJSONPath("data.period_start", "2026-10-01")
		past.AssertJSONPath("data.cost_micros", 0)
		past.AssertJSONPath("data.daily", []testkit.JSON{{"day": "2026-10-31", "cost_micros": 7000, "runs_count": 1}})
	})

	t.Run("answers an empty month with zeros", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.GetJSON("/api/v1/ai/usage").AssertOK()

		res.AssertJSONPath("data.runs_count", 0)
		res.AssertJSONPath("data.daily", []any{})
		res.AssertJSONPath("data.by_model", []any{})
	})

	t.Run("refuses a period that is no month", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON("/api/v1/ai/usage?period=last-week").AssertValidationError("period")
	})

	t.Run("lets members read usage", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/ai/usage").AssertOK()
	})
}
