package sla_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its ticket types and states read by name.
*/
type desk struct {
	tests.Desk
	types  map[string]string
	states map[string]string
}

/*
newDesk starts a workspace and reads its default ticket setup.
*/
func newDesk(t *testing.T) desk {
	t.Helper()

	d := desk{Desk: tests.NewDesk(t), types: map[string]string{}, states: map[string]string{}}
	types := d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()
	for i := range count(types) {
		d.types[types.String(at(i, "name"))] = types.String(at(i, "id"))
	}

	states := d.Owner.GetJSON("/api/v1/ticket-states").AssertOK()
	for i := range count(states) {
		d.states[states.String(at(i, "name"))] = states.String(at(i, "id"))
	}

	return d
}

/*
at is the JSON path of a field of a collection's item.
*/
func at(i int, field string) string {
	return "data." + strconv.Itoa(i) + "." + field
}

/*
count reads how many items a response's data holds.
*/
func count(res *testkit.Response) int {
	n, _ := strconv.Atoi(res.String("data.#"))

	return n
}

/*
policy creates a policy and returns its id.
*/
func (d desk) policy(t *testing.T, body testkit.JSON) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/sla-policies", body).AssertCreated().String("data.id")
}

/*
ticket starts a customer's conversation and converts it to a Question
ticket, returning the conversation and ticket ids.
*/
func (d desk) ticket(t *testing.T, name string) (string, string) {
	t.Helper()

	conv := d.Customer(t, name, "Refund please").String("data.id")
	ticket := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{
		"type_id": d.types["Question"],
	}).AssertCreated().String("data.id")

	return conv, ticket
}

/*
clocks returns a subject's clocks by target: state and due time.
*/
func (d desk) clocks(t *testing.T, path string) map[string]clock {
	t.Helper()

	var page struct {
		Data []clock `json:"data"`
	}
	d.Owner.GetJSON(path + "/sla-clocks").AssertOK().Decode(&page)

	out := map[string]clock{}
	for _, c := range page.Data {
		if _, seen := out[c.Target]; !seen {
			out[c.Target] = c
		}
	}

	return out
}

/*
clock is one clock as the API shows it.
*/
type clock struct {
	Target     string     `json:"target"`
	State      string     `json:"state"`
	DueAt      time.Time  `json:"due_at"`
	PausedAt   *time.Time `json:"paused_at"`
	BreachedAt *time.Time `json:"breached_at"`
}

/*
sweep runs the minute's SLA sweep and the jobs it queued.
*/
func (d desk) sweep(t *testing.T) {
	t.Helper()

	require.NoError(t, d.TC.App.Actions.SLA.Sweep(t.Context()))
	d.TC.Drain()
}

func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("creates, changes, reads and archives a policy", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		d.Owner.PostJSON("/api/v1/sla-policies", testkit.JSON{"name": "Empty"}).AssertStatus(422).
			AssertJSONPath("code", "sla_target_missing")
		d.Owner.PostJSON("/api/v1/sla-policies", testkit.JSON{"name": "Fast", "first_reply_seconds": 5}).
			AssertStatus(422).AssertJSONPath("code", "sla_target_invalid")

		id := d.policy(t, testkit.JSON{
			"name": "Standard", "first_reply_seconds": 3600, "resolution_seconds": 86400,
			"ticket_type_ids": []string{d.types["Question"]}, "breach_priority": "high",
		})
		d.Owner.GetJSON("/api/v1/sla-policies/"+id).AssertOK().AssertJSONPath("data.warn_before_seconds", 900.0).
			AssertJSONPath("data.pause_on_waiting_customer", true).AssertJSONPath("data.ticket_type_ids.0",
			d.types["Question"])

		d.Owner.PatchJSON("/api/v1/sla-policies/"+id, testkit.JSON{
			"first_reply_seconds": 0, "next_reply_seconds": 1800, "breach_priority": "none",
		}).AssertOK().AssertJSONPath("data.first_reply_seconds", nil).
			AssertJSONPath("data.next_reply_seconds", 1800.0).AssertJSONPath("data.breach_priority", nil)

		d.Owner.Delete("/api/v1/sla-policies/" + id).AssertNoContent()
		d.Owner.GetJSON("/api/v1/sla-policies").AssertOK().AssertJSONPath("data.0.id", id).
			AssertJSONPath("data.0.name", "Standard")
		assert.NotEmpty(t, d.Owner.GetJSON("/api/v1/sla-policies/"+id).String("data.archived_at"))
	})

	t.Run("refuses a schedule of another workspace and members who may not set tickets up", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		d.Owner.PostJSON("/api/v1/sla-policies", testkit.JSON{
			"name": "Bad", "first_reply_seconds": 60, "schedule_id": "sch_nothing",
		}).AssertStatus(422).AssertJSONPath("code", "schedule_not_found")

		member, _ := d.Join(t, enums.RoleMember)
		member.PostJSON("/api/v1/sla-policies", testkit.JSON{"name": "Mine", "first_reply_seconds": 60}).
			AssertStatus(403)
	})
}

func TestTicketClocks(t *testing.T) {
	t.Parallel()

	t.Run("times replies on a matching ticket and escalates a breach", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Escalations"}).AssertCreated().
			String("data.id")
		policy := d.policy(t, testkit.JSON{
			"name": "Questions", "first_reply_seconds": 3600, "next_reply_seconds": 1800,
			"resolution_seconds": 86400, "ticket_type_ids": []string{d.types["Question"]},
			"breach_priority": "urgent", "breach_team_id": team,
		})
		breached := tests.Record[appevents.TicketSLABreached](d.TC)

		conv, ticket := d.ticket(t, "ada")
		d.Owner.GetJSON("/api/v1/tickets/"+ticket).AssertOK().AssertJSONPath("data.sla_policy_id", policy)

		clocks := d.clocks(t, "/api/v1/tickets/"+ticket)
		require.Equal(t, "running", clocks["first_reply"].State)
		assert.Equal(t, tests.Epoch().Add(time.Hour), clocks["first_reply"].DueAt.UTC())
		assert.Equal(t, "running", clocks["resolution"].State)

		d.TC.Clock.Travel(10 * time.Minute)
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/replies", testkit.JSON{"body": "On it."}).AssertCreated()
		assert.Equal(t, "met", d.clocks(t, "/api/v1/tickets/"+ticket)["first_reply"].State)

		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/messages", testkit.JSON{"body": "Any news?"}).
			AssertCreated()
		require.Equal(t, "running", d.clocks(t, "/api/v1/tickets/"+ticket)["next_reply"].State)

		d.TC.Clock.Travel(31 * time.Minute)
		d.sweep(t)

		clocks = d.clocks(t, "/api/v1/tickets/"+ticket)
		assert.Equal(t, "breached", clocks["next_reply"].State)
		d.Owner.GetJSON("/api/v1/tickets/"+ticket).AssertOK().AssertJSONPath("data.priority", "urgent").
			AssertJSONPath("data.team_id", team)
		d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.priority", "urgent")
		require.Len(t, breached.All(), 1)
		assert.Equal(t, "next_reply", breached.All()[0].Changes["target"].To)

		d.Owner.GetJSON("/api/v1/conversations/"+conv+"/sla-clocks").AssertOK().
			AssertJSONPath("data.0.ticket_id", ticket)
	})

	t.Run("warns once before a clock is due", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.policy(t, testkit.JSON{
			"name": "Warned", "first_reply_seconds": 3600, "warn_before_seconds": 600,
			"ticket_type_ids": []string{d.types["Question"]},
		})
		warned := tests.Record[appevents.TicketSLAWarning](d.TC)

		_, ticket := d.ticket(t, "bo")
		d.TC.Clock.Travel(51 * time.Minute)
		d.sweep(t)
		d.sweep(t)

		require.Len(t, warned.All(), 1)
		assert.Equal(t, "running", d.clocks(t, "/api/v1/tickets/"+ticket)["first_reply"].State)
	})

	t.Run("pauses while waiting on the customer and moves the due time on resuming", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.policy(t, testkit.JSON{
			"name": "Resolve", "resolution_seconds": 4 * 3600, "ticket_type_ids": []string{d.types["Question"]},
		})

		_, ticket := d.ticket(t, "cy")
		path := "/api/v1/tickets/" + ticket
		due := d.clocks(t, path)["resolution"].DueAt

		d.TC.Clock.Travel(time.Hour)
		d.Owner.PatchJSON(path, testkit.JSON{"state_id": d.states["Needs info"]}).AssertOK()
		assert.Equal(t, "paused", d.clocks(t, path)["resolution"].State)

		d.TC.Clock.Travel(2 * time.Hour)
		d.Owner.PatchJSON(path, testkit.JSON{"state_id": d.states["Investigating"]}).AssertOK()
		resumed := d.clocks(t, path)["resolution"]
		assert.Equal(t, "running", resumed.State)
		assert.Equal(t, due.Add(2*time.Hour).UTC(), resumed.DueAt.UTC())

		d.Owner.PatchJSON(path, testkit.JSON{"state_id": d.states["Resolved"]}).AssertOK()
		assert.Equal(t, "met", d.clocks(t, path)["resolution"].State)
	})

	t.Run("counts only the policy's open hours", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		week := testkit.JSON{}
		for _, day := range []string{"mon", "tue", "wed", "thu", "fri"} {
			week[day] = [][]string{{"09:00", "17:00"}}
		}

		schedule := d.Owner.PostJSON("/api/v1/schedules", testkit.JSON{
			"name": "Support hours", "timezone": "UTC", "weekly": week,
		}).AssertCreated().String("data.id")
		d.policy(t, testkit.JSON{
			"name": "Business", "first_reply_seconds": 10 * 3600, "schedule_id": schedule,
			"ticket_type_ids": []string{d.types["Question"]},
		})

		_, ticket := d.ticket(t, "di")
		due := d.clocks(t, "/api/v1/tickets/"+ticket)["first_reply"].DueAt
		assert.Equal(t, time.Date(2026, 11, 3, 11, 0, 0, 0, time.UTC), due.UTC())
	})

	t.Run("archiving a policy cancels its running clocks", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		policy := d.policy(t, testkit.JSON{
			"name": "Gone", "first_reply_seconds": 3600, "ticket_type_ids": []string{d.types["Question"]},
		})

		_, ticket := d.ticket(t, "ed")
		d.Owner.Delete("/api/v1/sla-policies/" + policy).AssertNoContent()
		assert.Equal(t, "canceled", d.clocks(t, "/api/v1/tickets/"+ticket)["first_reply"].State)
	})
}
