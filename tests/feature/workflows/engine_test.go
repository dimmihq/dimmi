package workflows_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
live creates a workflow from the definition and publishes it.
*/
func live(t *testing.T, d tests.Desk, name string, spec testkit.JSON) {
	t.Helper()

	id := create(t, d, testkit.JSON{"name": name, "spec": spec})
	d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()
}

/*
conversation starts a conversation from a customer and runs the jobs it
queued, returning its API path.
*/
func conversation(t *testing.T, d tests.Desk, name, body string) string {
	t.Helper()

	id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": "Help", "body": body,
		"contact": testkit.JSON{"email": name + "@example.com", "name": name},
	}).AssertCreated().String("data.id")
	d.TC.Drain()

	return "/api/v1/conversations/" + id
}

/*
bodies returns the bodies of a conversation's messages, oldest first.
*/
func bodies(t *testing.T, d tests.Desk, path string) []string {
	t.Helper()

	var page struct {
		Data []struct {
			Type string  `json:"type"`
			Body *string `json:"body"`
		} `json:"data"`
	}
	d.Owner.GetJSON(path + "/messages?order=asc").AssertOK().Decode(&page)

	var out []string
	for _, m := range page.Data {
		if m.Type != "event" && m.Body != nil {
			out = append(out, *m.Body)
		}
	}

	return out
}

/*
step is one step of a definition.
*/
func step(id, kind string, settings testkit.JSON, paths ...testkit.JSON) testkit.JSON {
	s := testkit.JSON{"id": id, "kind": kind, "settings": settings}
	if len(paths) > 0 {
		s["paths"] = paths
	}

	return s
}

/*
path is one path of a step.
*/
func path(id, label string, steps ...testkit.JSON) testkit.JSON {
	if steps == nil {
		steps = []testkit.JSON{}
	}

	return testkit.JSON{"id": id, "label": label, "steps": steps}
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("routes a first message with buttons and the customer's answer", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		live(t, d, "Route by topic", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_ask0001", "buttons",
				testkit.JSON{"text": "Hi {{contact.first_name}}, what is it about?"},
				path("p_billing", "Billing",
					step("s_assign1", "assign", testkit.JSON{"mode": "team", "team_id": team}),
					step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}}),
					step("s_thanks1", "message", testkit.JSON{"text": "Thanks, billing will reply soon."})),
				path("p_other1", "Something else"))},
		})

		conv := conversation(t, d, "ada", "Hello there")
		got := bodies(t, d, conv)
		require.Len(t, got, 2)
		assert.Equal(t, "Hi ada, what is it about?\n\n1. Billing\n2. Something else", got[1])

		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "it's about billing please"}).AssertCreated()
		d.TC.Drain()

		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.team.id", team).
			AssertJSONPath("data.tags.0.id", tag)
		got = bodies(t, d, conv)
		assert.Equal(t, "Thanks, billing will reply soon.", got[len(got)-1])
	})

	t.Run("asks again once when an answer names no choice, then gives up", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Menu", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_ask0001", "buttons",
				testkit.JSON{"text": "Pick one", "allow_typing": false},
				path("p_aaaaaa", "Billing"), path("p_bbbbbb", "Bugs"))},
		})

		conv := conversation(t, d, "bo", "Hi")
		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "hmm"}).AssertCreated()
		d.TC.Drain()
		got := bodies(t, d, conv)
		assert.True(t, strings.HasPrefix(got[len(got)-1], "Please reply with one of the numbers above."))

		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "still no"}).AssertCreated()
		d.TC.Drain()
		assert.Len(t, bodies(t, d, conv), len(got)+1)
	})

	t.Run("runs background workflows on customer messages that match", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "refund"}).AssertCreated().String("data.id")
		live(t, d, "Refunds", testkit.JSON{
			"schema": 1, "trigger": "message.customer",
			"steps": []testkit.JSON{step("s_branch1", "branch", testkit.JSON{},
				testkit.JSON{"id": "p_refund1", "label": "Refund", "steps": []testkit.JSON{
					step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}}),
					step("s_prio001", "priority", testkit.JSON{"value": "high"}),
				}, "condition": testkit.JSON{
					"field": "message.body", "op": "contains_any", "value": []string{"refund"},
				}},
				testkit.JSON{"id": "p_else001", "outcome": "else", "steps": []testkit.JSON{}})},
		})

		conv := conversation(t, d, "cy", "Hello")
		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "I want a Refund now"}).AssertCreated()
		d.TC.Drain()

		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.tags.0.id", tag).AssertJSONPath("data.priority", "high")
	})

	t.Run("runs a workflow kept to once per conversation the first time, and not again", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Once", testkit.JSON{
			"schema": 1, "trigger": "message.customer", "reentry": testkit.JSON{"mode": "once_per_conversation"},
			"steps": []testkit.JSON{step("s_prio001", "priority", testkit.JSON{"value": "high"})},
		})
		conv := conversation(t, d, "cy", "Hello")

		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "Anyone there?"}).AssertCreated()
		d.TC.Drain()
		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.priority", "high")
		d.Owner.PatchJSON(conv, testkit.JSON{"priority": "low"}).AssertOK()
		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "Still there?"}).AssertCreated()
		d.TC.Drain()

		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.priority", "low")
	})

	t.Run("stops talking once a teammate replies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Menu", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_ask0001", "buttons", testkit.JSON{"text": "Pick one"},
				path("p_aaaaaa", "Billing", step("s_msg0001", "message", testkit.JSON{"text": "Routing you"})),
				path("p_bbbbbb", "Bugs"))},
		})

		conv := conversation(t, d, "di", "Hi")
		d.Owner.PostJSON(conv+"/replies", testkit.JSON{"body": "I'm here, what's up?"}).AssertCreated()
		d.TC.Drain()
		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "1"}).AssertCreated()
		d.TC.Drain()

		assert.NotContains(t, bodies(t, d, conv), "Routing you")
	})

	t.Run("runs status workflows only for the statuses they name", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "On close", testkit.JSON{
			"schema": 1, "trigger": "conversation.status_changed",
			"trigger_settings": testkit.JSON{"to": []string{"closed"}},
			"steps": []testkit.JSON{
				step("s_note001", "note", testkit.JSON{"text": "closed by {{conversation.number}}"}),
			},
		})

		conv := conversation(t, d, "fy", "Hi")
		d.Owner.PatchJSON(conv, testkit.JSON{"status": "snoozed", "snooze_preset": "next_week"}).AssertOK()
		d.TC.Drain()
		assert.NotContains(t, bodies(t, d, conv), "closed by 1")

		d.Owner.PatchJSON(conv, testkit.JSON{"status": "closed"}).AssertOK()
		d.TC.Drain()
		assert.Contains(t, bodies(t, d, conv), "closed by 1")
	})

	t.Run("never starts a workflow from its own changes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Other"}).AssertCreated().String("data.id")
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Ping"}).AssertCreated().String("data.id")
		live(t, d, "Flip", testkit.JSON{
			"schema": 1, "trigger": "conversation.assignment_changed",
			"steps": []testkit.JSON{
				step("s_assign1", "assign", testkit.JSON{"mode": "team", "team_id": team, "keep_if_assigned": false}),
				step("s_note001", "note", testkit.JSON{"text": "flipped"}),
			},
		})

		conv := conversation(t, d, "ed", "Hi")
		d.Owner.PatchJSON(conv, testkit.JSON{"team_id": other}).AssertOK()
		d.TC.Drain()
		d.TC.Drain()

		notes := 0
		for _, b := range bodies(t, d, conv) {
			if b == "flipped" {
				notes++
			}
		}

		assert.Equal(t, 1, notes)
	})
}
