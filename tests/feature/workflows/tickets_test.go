package workflows_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
ticketSetup reads the workspace's ticket types and states by name.
*/
func ticketSetup(t *testing.T, d tests.Desk) (map[string]string, map[string]string) {
	t.Helper()

	byName := func(path string) map[string]string {
		res := d.Owner.GetJSON(path).AssertOK()
		n, _ := strconv.Atoi(res.String("data.#"))
		out := map[string]string{}

		for i := range n {
			out[res.String("data."+strconv.Itoa(i)+".name")] = res.String("data." + strconv.Itoa(i) + ".id")
		}

		return out
	}

	return byName("/api/v1/ticket-types"), byName("/api/v1/ticket-states")
}

/*
ownTicket returns the conversation's ticket, empty for none.
*/
func ownTicket(t *testing.T, d tests.Desk, conv string) *testkit.Response {
	t.Helper()

	return d.Owner.GetJSON(conv + "/tickets").AssertOK()
}

/*
runs returns a workflow's runs, newest first.
*/
func runs(t *testing.T, d tests.Desk, workflow string) *testkit.Response {
	t.Helper()

	return d.Owner.GetJSON("/api/v1/workflows/" + workflow + "/runs").AssertOK()
}

/*
convert makes the conversation a ticket of the type.
*/
func convert(t *testing.T, d tests.Desk, conv, typeID string) string {
	t.Helper()

	id := d.Owner.PostJSON(conv+"/ticket", testkit.JSON{"type_id": typeID}).AssertCreated().String("data.id")
	d.TC.Drain()

	return id
}

func TestTicketSteps(t *testing.T) {
	t.Parallel()

	t.Run("makes a conversation a ticket with values mapped from variables", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		types, _ := ticketSetup(t, d)
		live(t, d, "File bugs", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_ticket1", "create_ticket", testkit.JSON{
				"type_id": types["Bug report"], "title": "Bug from {{contact.name}}",
				"fields": []testkit.JSON{
					{"key": "severity", "value": "high"}, {"key": "steps", "value": "{{conversation.subject}}"},
				},
			})},
		})

		conv := conversation(t, d, "ada", "The export crashes")
		own := ownTicket(t, d, conv)
		own.AssertJSONPath("data.ticket.title", "Bug from ada").AssertJSONPath("data.ticket.fields.severity", "high").
			AssertJSONPath("data.ticket.fields.steps", "Help")
	})

	t.Run("records a refused ticket as a failed step and goes on", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		types, _ := ticketSetup(t, d)
		d.Owner.PostJSON("/api/v1/ticket-types/"+types["Question"]+"/fields", testkit.JSON{
			"key": "order_number", "label": "Order number", "data_type": "short_text", "required_create": true,
		}).AssertCreated()
		wf := publish(t, d, "Questions", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_ticket1", "create_ticket", testkit.JSON{"type_id": types["Question"]}),
				note("s_note001", "Still here"),
			},
		})

		conv := conversation(t, d, "bo", "Where is my order?")
		ownTicket(t, d, conv).AssertJSONMissing("data.ticket.id")

		res := runs(t, d, wf)
		res.AssertJSONPath("data.0.status", "completed").
			AssertJSONPath(`data.0.steps.#(step_id=="s_ticket1").status`, "failed").
			AssertJSONPath(`data.0.steps.#(step_id=="s_ticket1").outcome`, "required_fields_missing").
			AssertJSONPath(`data.0.steps.#(step_id=="s_ticket1").detail.missing.0`, "order_number").
			AssertJSONPath(`data.0.steps.#(step_id=="s_note001").status`, "completed")
	})

	t.Run("escalates a new ticket and adds it to a tracker", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		types, _ := ticketSetup(t, d)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": types["Tracker"], "title": "Refunds are slow",
		}).AssertCreated().String("data.id")
		live(t, d, "Route refunds", testkit.JSON{
			"schema": 1, "trigger": "ticket.created",
			"steps": []testkit.JSON{
				step("s_track01", "add_to_tracker", testkit.JSON{"tracker_id": tracker}),
				step("s_escal01", "escalate", testkit.JSON{
					"type_id": types["Back-office task"], "title": "Refund for {{ticket.reference}}",
				}),
			},
		})

		conv := conversation(t, d, "cy", "Refund")
		ticket := convert(t, d, conv, types["Question"])

		d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/conversations").AssertOK().
			AssertJSONPath("meta.impact.customers", float64(1))
		shown := d.Owner.GetJSON("/api/v1/tickets/" + ticket).AssertOK()
		shown.AssertJSONPath("data.group", "waiting_team").
			AssertJSONPath(`data.links.#(kind=="back_office").ticket.title`,
				"Refund for "+shown.String("data.reference"))
	})

	t.Run("applies an SLA to a plain conversation and runs breach workflows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		policy := d.Owner.PostJSON("/api/v1/sla-policies", testkit.JSON{
			"name": "Chat", "first_reply_seconds": 600,
		}).AssertCreated().String("data.id")
		live(t, d, "Time chats", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_sla0001", "apply_sla", testkit.JSON{"sla_policy_id": policy})},
		})
		live(t, d, "On breach", testkit.JSON{
			"schema": 1, "trigger": "sla.breached",
			"steps": []testkit.JSON{
				step("s_prio001", "priority", testkit.JSON{"value": "urgent"}), note("s_note001", "SLA missed"),
			},
		})

		conv := conversation(t, d, "di", "Hello?")
		d.Owner.GetJSON(conv+"/sla-clocks").AssertOK().AssertJSONPath("data.0.target", "first_reply").
			AssertJSONPath("data.0.state", "running").AssertJSONPath("data.0.ticket_id", nil)

		d.TC.Clock.Travel(11 * time.Minute)
		require.NoError(t, d.TC.App.Actions.SLA.Sweep(t.Context()))
		d.TC.Drain()

		d.Owner.GetJSON(conv+"/sla-clocks").AssertOK().AssertJSONPath("data.0.state", "breached")
		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.priority", "urgent")
		got := bodies(t, d, conv)
		assert.Equal(t, "SLA missed", got[len(got)-1])
	})
}

func TestTicketTriggers(t *testing.T) {
	t.Parallel()

	t.Run("closes the loop with a workflow message once the work behind it is done", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		types, states := ticketSetup(t, d)
		live(t, d, "Close the loop", testkit.JSON{
			"schema": 1, "trigger": "ticket.close_loop",
			"steps": []testkit.JSON{
				step("s_tell001", "message", testkit.JSON{"text": "Good news: {{ticket.reference}} is done."}),
				step("s_state01", "set_ticket_state", testkit.JSON{"state_id": states["Resolved"]}),
			},
		})

		conv := conversation(t, d, "ed", "Refund")
		ticket := convert(t, d, conv, types["Question"])
		work := d.Owner.PostJSON("/api/v1/tickets/"+ticket+"/escalations", testkit.JSON{
			"type_id": types["Back-office task"], "title": "Refund",
		}).AssertCreated().String("data.id")

		d.Owner.PatchJSON("/api/v1/tickets/"+work, testkit.JSON{"state_id": states["Resolved"]}).AssertOK()
		d.TC.Drain()

		shown := d.Owner.GetJSON("/api/v1/tickets/" + ticket).AssertOK()
		shown.AssertJSONPath("data.group", "resolved")
		got := bodies(t, d, conv)
		assert.Contains(t, got, "Good news: "+shown.String("data.reference")+" is done.")
	})

	t.Run("reads ticket facts in branches and never re-runs on its own changes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		types, _ := ticketSetup(t, d)
		wf := publish(t, d, "Severity", testkit.JSON{
			"schema": 1, "trigger": "ticket.field_changed",
			"steps": []testkit.JSON{step("s_branch1", "branch", nil,
				testkit.JSON{
					"id": "p_bugs001", "label": "Bug reports",
					"condition": testkit.JSON{"all": []testkit.JSON{
						{"field": "ticket.type_id", "op": "eq", "value": types["Bug report"]},
						{"field": "trigger.fields", "op": "has", "value": "steps"},
					}},
					"steps": []testkit.JSON{step("s_fields1", "set_ticket_fields", testkit.JSON{
						"fields": []testkit.JSON{{"key": "severity", "value": "high"}},
					})},
				},
				testkit.JSON{"id": "p_else001", "outcome": "else", "steps": []testkit.JSON{}},
			)},
		})

		conv := conversation(t, d, "fi", "Crash")
		ticket := convert(t, d, conv, types["Bug report"])
		d.Owner.PatchJSON("/api/v1/tickets/"+ticket, testkit.JSON{"fields": testkit.JSON{"steps": "Open, click"}}).
			AssertOK()
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/tickets/"+ticket).AssertOK().AssertJSONPath("data.fields.severity", "high")
		res := runs(t, d, wf)
		res.AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.trigger", "ticket.field_changed")
	})
}
