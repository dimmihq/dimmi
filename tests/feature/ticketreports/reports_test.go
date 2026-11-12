package ticketreports_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its ticket types and states by name.
*/
type desk struct {
	tests.Desk
	types  map[string]string
	states map[string]string
}

/*
newDesk starts a workspace and reads its ticket setup.
*/
func newDesk(t *testing.T) desk {
	t.Helper()

	d := desk{Desk: tests.NewDesk(t), types: map[string]string{}, states: map[string]string{}}
	types := d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()

	for i := range n(types, "data.#") {
		d.types[types.String(at(i, "name"))] = types.String(at(i, "id"))
	}

	states := d.Owner.GetJSON("/api/v1/ticket-states").AssertOK()
	for i := range n(states, "data.#") {
		d.states[states.String(at(i, "name"))] = states.String(at(i, "id"))
	}

	return d
}

func n(res *testkit.Response, path string) int {
	v, _ := strconv.Atoi(res.String(path))

	return v
}

func at(i int, field string) string {
	return "data." + strconv.Itoa(i) + "." + field
}

/*
report turns a new customer conversation into a bug report of the
severity and returns the ticket's id.
*/
func (d desk) report(t *testing.T, name, severity string) string {
	t.Helper()

	conv := d.Customer(t, name, "Export fails for "+name).String("data.id")
	id := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{
		"type_id": d.types["Bug report"], "fields": testkit.JSON{"severity": severity},
	}).AssertCreated().String("data.id")

	return id
}

/*
decode reads a public id.
*/
func (d desk) decode(t *testing.T, prefix models.Prefix, id string) ids.UUID {
	t.Helper()

	out, err := d.TC.App.IDs.Decode(string(prefix), id)
	require.NoError(t, err)

	return out
}

/*
exec runs SQL as the operator.
*/
func (d desk) exec(t *testing.T, sql string, args ...any) {
	t.Helper()

	_, err := d.TC.Operator().Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func TestTicketReports(t *testing.T) {
	t.Parallel()

	t.Run("time in state, tickets by type and by field", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		first := d.report(t, "ana", "high")
		d.report(t, "ben", "low")

		d.TC.Clock.Travel(2 * time.Hour)
		d.Owner.PatchJSON("/api/v1/tickets/"+first, testkit.JSON{"state_id": d.states["Investigating"]}).AssertOK()
		d.TC.Clock.Travel(3 * time.Hour)
		d.Owner.PatchJSON("/api/v1/tickets/"+first, testkit.JSON{"state_id": d.states["Resolved"]}).AssertOK()
		d.TC.Drain()

		times := d.Owner.GetJSON("/api/v1/reports/tickets/time-in-state").AssertOK()
		row := `data.#(type_name=="Bug report")#|#(state_name=="New")`
		assert.Equal(t, "2", times.String(row+".tickets"))
		assert.Equal(t, "7200", times.String(row+".median_seconds"))
		assert.Equal(t, "18000", times.String(row+".p90_seconds"))
		assert.Equal(t, "10800", times.String(`data.#(state_name=="Investigating").median_seconds`))
		times.AssertJSONMissing(`data.#(state_name=="Resolved")`)

		types := d.Owner.GetJSON("/api/v1/reports/tickets/by-type").AssertOK()
		assert.Equal(t, "2", types.String(`data.#(name=="Bug report").created`))
		assert.Equal(t, "1", types.String(`data.#(name=="Bug report").resolved`))
		assert.Equal(t, "1", types.String(`data.#(name=="Bug report").open`))
		assert.Equal(t, "0", types.String(`data.#(name=="Question").created`))

		fields := d.Owner.GetJSON("/api/v1/reports/tickets/by-field?type_id=" + d.types["Bug report"] +
			"&field=severity").AssertOK()
		assert.Equal(t, "1", fields.String(`data.values.#(value=="high").tickets`))
		assert.Equal(t, "1", fields.String(`data.values.#(value=="high").resolved`))
		assert.Equal(t, "1", fields.String(`data.values.#(value=="low").tickets`))

		d.Owner.GetJSON("/api/v1/reports/tickets/by-field?type_id=" + d.types["Bug report"] + "&field=nope").
			AssertStatus(404)
	})

	t.Run("a range before the tickets counts none", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.report(t, "ana", "high")

		old := d.TC.Clock.Now().AddDate(0, 0, -60).Format(time.DateOnly)
		res := d.Owner.GetJSON("/api/v1/reports/tickets/by-type?from=" + old + "&to=" + old).AssertOK()
		assert.Equal(t, "0", res.String(`data.#(name=="Bug report").created`))

		d.Owner.GetJSON("/api/v1/reports/tickets/sla?from=2026-13-01").AssertStatus(422)
	})

	t.Run("service levels kept per policy and target", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ticket := d.report(t, "ana", "high")
		id := d.decode(t, models.PrefixTicket, ticket)
		ws, now := d.TC.Workspace.Workspace.ID, d.TC.Clock.Now()

		policy := ids.New()
		d.exec(t, `INSERT INTO sla_policies (id, workspace_id, name, first_reply_seconds)
			VALUES ($1, $2, 'Gold', 3600)`, policy, ws)
		d.exec(t, `INSERT INTO sla_clocks (workspace_id, policy_id, ticket_id, target, state, started_at, due_at,
			stopped_at) VALUES ($1, $2, $3, 'first_reply', 'met', $4, $5, $6)`,
			ws, policy, id, now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-150*time.Minute))
		d.exec(t, `INSERT INTO sla_clocks (workspace_id, policy_id, ticket_id, target, state, started_at, due_at,
			breached_at) VALUES ($1, $2, $3, 'resolution', 'breached', $4, $5, $5)`,
			ws, policy, id, now.Add(-3*time.Hour), now.Add(-time.Hour))
		d.exec(t, `INSERT INTO sla_clocks (workspace_id, policy_id, ticket_id, target, state, started_at, due_at)
			VALUES ($1, $2, $3, 'next_reply', 'running', $4, $5)`,
			ws, policy, id, now.Add(-time.Hour), now.Add(time.Hour))

		res := d.Owner.GetJSON("/api/v1/reports/tickets/sla").AssertOK()
		assert.Equal(t, "1", res.String(`data.#(target=="first_reply").met`))
		assert.Equal(t, "100", res.String(`data.#(target=="first_reply").hit_rate`))
		assert.Equal(t, "1800", res.String(`data.#(target=="first_reply").mean_met_seconds`))
		assert.Equal(t, "0", res.String(`data.#(target=="resolution").hit_rate`))
		assert.Equal(t, "1", res.String(`data.#(target=="next_reply").running`))
		assert.Equal(t, "Gold", res.String(`data.#(target=="next_reply").policy_name`))

		report := d.Owner.GetJSON("/api/v1/reports/tickets").AssertOK()
		assert.Equal(t, "50", report.String(`data.metrics.#(key=="sla_hit_rate").value`))
		assert.Equal(t, "1", report.String(`data.metrics.#(key=="tickets_created").value`))
		assert.NotEmpty(t, report.String(`data.charts.#(key=="time_in_state")`+".key"))
		assert.NotEmpty(t, report.String(`data.charts.#(key=="tracker_impact")`+".key"))
		assert.NotEmpty(t, report.String(`data.charts.#(title=="Bug report by Severity")`+".key"))
	})

	t.Run("ranks trackers by the revenue they reach", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ws := d.TC.Workspace.Workspace.ID

		big := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "Exports time out",
		}).AssertCreated().String("data.id")
		small := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "Typo on billing page",
		}).AssertCreated().String("data.id")

		for i, spend := range []int{1000, 250} {
			name := []string{"ana", "ben"}[i]
			conv := d.Customer(t, name, "Export").String("data.id")
			company := ids.New()
			d.exec(t, `INSERT INTO companies (id, workspace_id, name, custom_attributes)
				VALUES ($1, $2, $3, jsonb_build_object('monthly_spend', $4::int))`, company, ws, name+" Inc", spend)
			d.exec(t, `UPDATE contacts SET company_id = $1 WHERE workspace_id = $2 AND email = $3`,
				company, ws, name+"@example.com")
			d.Owner.PostJSON("/api/v1/tickets/"+big+"/links", testkit.JSON{"kind": "tracker", "conversation_id": conv}).
				AssertOK()
		}

		res := d.Owner.GetJSON("/api/v1/reports/tickets/trackers").AssertOK()
		assert.Equal(t, big, res.String("data.0.id"))
		assert.Equal(t, "2", res.String("data.0.customers"))
		assert.Equal(t, "2", res.String("data.0.companies"))
		assert.Equal(t, "2", res.String("data.0.conversations_in_range"))
		assert.Equal(t, "1250", res.String("data.0.revenue"))
		assert.Equal(t, small, res.String("data.1.id"))
		assert.Equal(t, "0", res.String("data.1.revenue"))
	})
}
