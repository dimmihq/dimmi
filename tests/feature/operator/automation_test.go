package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
workflow creates the draft workflow Triage on the first message, with
one note step, and returns its id.
*/
func (k *desk) workflow() string {
	k.t.Helper()
	return k.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Triage", "spec": testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{{"id": "s_note0001", "kind": "note", "settings": testkit.JSON{"text": "triage"}}},
	}}).AssertCreated().String("data.id")
}

/*
lastToolResult is what the last tool call before the latest model call
returned.
*/
func (k *desk) lastToolResult() string {
	k.t.Helper()
	all := k.llm.Sent()
	sent := all[len(all)-1].Messages
	return sent[len(sent)-1].Parts[0].Text
}

/*
toolResultAt is what the last tool call before the model call at index
i returned.
*/
func (k *desk) toolResultAt(i int) string {
	k.t.Helper()
	sent := k.llm.Sent()[i].Messages
	return sent[len(sent)-1].Parts[0].Text
}

/*
TestOperatorWorkflows covers Operator reading, editing and drafting
workflows, and explaining what they did.
*/
func TestOperatorWorkflows(t *testing.T) {
	t.Parallel()

	t.Run("suggests a step on the working copy, applies it in the builder's place and undoes it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		wf := k.workflow()
		thread := k.thread("")
		k.llm.Script(tools(call("propose_workflow_edit", map[string]any{
			"suggestion_title": "Mark it high", "reason": "These are urgent.", "workflow_id": wf,
			"edits": []map[string]any{{
				"action": "add_step", "after_step_id": "s_note0001",
				"step": map[string]any{"kind": "priority", "settings": map[string]any{"value": "high"}},
			}},
		})), reply("Suggested."))

		res := k.ask(thread, "Make these high priority").AssertOK()
		id := res.String("data.changesets.0.id")
		res.AssertJSONPath("data.changesets.0.operations.0.kind", "workflow.edit_draft").
			AssertJSONPath("data.changesets.0.operations.0.workflow_id", wf)
		assert.Contains(t, res.String("data.changesets.0.operations.0.text"), `"kind": "priority"`)
		back := k.lastToolResult()
		assert.Contains(t, back, "The workflow would read:")
		assert.Contains(t, back, "step s_note0001 · note")
		assert.Contains(t, back, "· priority · settings {\"value\":\"high\"}")
		k.d.Owner.GetJSON("/api/v1/workflows/"+wf).AssertOK().AssertJSONPath("data.spec.steps.#", float64(1))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		shown := k.d.Owner.GetJSON("/api/v1/workflows/" + wf).AssertOK()
		shown.AssertJSONPath("data.spec.steps.#", float64(2)).AssertJSONPath("data.spec.steps.1.kind", "priority").
			AssertJSONPath("data.status", "draft")

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "reverted")
		k.d.Owner.GetJSON("/api/v1/workflows/"+wf).AssertOK().AssertJSONPath("data.spec.steps.#", float64(1))
	})

	t.Run("refuses an edit publishing would refuse, saying what to fix", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		wf := k.workflow()
		k.llm.Script(tools(call("propose_workflow_edit", map[string]any{
			"suggestion_title": "Greet", "reason": "Say hello.", "workflow_id": wf,
			"edits": []map[string]any{
				{"action": "add_step", "at_start": true, "step": map[string]any{"kind": "message"}},
				{"action": "remove_step", "step_id": "s_nope0001"},
			},
		})), reply("Could not."))

		k.ask(k.thread(""), "Greet them").AssertOK().AssertJSONPath("data.changesets.#", float64(0))
		assert.Contains(t, k.lastToolResult(), `edit 2 (remove_step): the workflow has no step "s_nope0001"`)

		k.llm.Script(tools(call("propose_workflow_edit", map[string]any{
			"suggestion_title": "Greet", "reason": "Say hello.", "workflow_id": wf,
			"edits": []map[string]any{{
				"action": "add_step", "at_start": true, "step": map[string]any{"kind": "message"},
			}},
		})), reply("Could not."))
		k.ask(k.thread(""), "Greet them").AssertOK().AssertJSONPath("data.changesets.#", float64(0))
		back := k.lastToolResult()
		assert.Contains(t, back, "publishing would refuse this")
		assert.Contains(t, back, "missing_setting: Write the text.")
		assert.Equal(t, 0, k.count("SELECT count(*) FROM operator_changesets"))
	})

	t.Run("drafts a new workflow from a description, created only once accepted", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(
			tools(call("get_workflow_reference", map[string]any{"trigger": "sla.breached"})),
			tools(call("propose_new_workflow", map[string]any{
				"suggestion_title": "Flag breached SLAs", "reason": "Breaches need a person.",
				"name": "Breached SLA", "trigger": "sla.breached",
				"spec": map[string]any{"steps": []map[string]any{
					{"kind": "priority", "settings": map[string]any{"value": "urgent"}},
					{"kind": "note", "settings": map[string]any{"text": "SLA breached."}},
				}},
			})),
			reply("Drafted."),
		)

		res := k.ask(k.thread(""), "When an SLA breaches, mark it urgent and leave a note").AssertOK()
		reference := k.toolResultAt(1)
		assert.Contains(t, reference, "- sla.breached: SLA breached.")
		assert.Contains(t, reference, "- priority: settings {value: string}")
		assert.NotContains(t, reference, "- conversation.opener:")
		res.AssertJSONPath("data.changesets.0.operations.0.kind", "workflow.create_draft")
		assert.Equal(t, 0, k.count("SELECT count(*) FROM workflows WHERE name = 'Breached SLA'"))

		applied := k.d.Owner.PostJSON("/api/v1/operator/changesets/"+res.String("data.changesets.0.id")+
			"/applications", testkit.JSON{}).AssertOK()
		created := applied.String("data.operations.0.created_id")
		require.NotEmpty(t, created)
		k.d.Owner.GetJSON("/api/v1/workflows/"+created).AssertOK().AssertJSONPath("data.trigger", "sla.breached").
			AssertJSONPath("data.status", "draft").AssertJSONPath("data.spec.steps.#", float64(2))
	})

	t.Run("reads the open workflow into the question, and edits on top of its open suggestion", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		wf := k.workflow()
		thread := k.thread("")
		k.llm.Script(tools(call("propose_workflow_edit", map[string]any{
			"suggestion_title": "Mark it high", "reason": "Urgent.", "workflow_id": wf,
			"edits": []map[string]any{{
				"action": "change_step", "step_id": "s_note0001",
				"settings": map[string]any{"text": "triage now"},
			}},
		})), reply("Suggested."))
		suggestion := k.ask(thread, "Reword the note").AssertOK().String("data.changesets.0.id")

		k.llm.Script(tools(call("propose_workflow_edit", map[string]any{
			"suggestion_title": "Mark it high", "reason": "Also high.", "workflow_id": wf,
			"edits": []map[string]any{{"action": "add_step", "step": map[string]any{
				"kind": "priority", "settings": map[string]any{"value": "high"},
			}}},
		})), reply("Added."))
		second := k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body":    "Also make it high",
			"viewing": testkit.JSON{"page": "workflow", "id": wf, "suggestion_id": suggestion},
		}).AssertOK()

		second.AssertJSONPath("data.changesets.#", float64(0))
		sent := k.llm.Sent()
		asked := sent[2].Messages[len(sent[2].Messages)-1]
		note := asked.Parts[len(asked.Parts)-2].Text
		assert.Contains(t, note, "The workflow builder is open on this workflow.")
		assert.Contains(t, note, "workflow_id: "+wf)
		assert.Contains(t, note, `settings {"text":"triage now"}`)
		list := k.d.Owner.GetJSON("/api/v1/operator/changesets?filter[status]=pending").AssertOK()
		list.AssertJSONPath("data.#", float64(1))
		text := list.String("data.0.operations.0.text")
		assert.Contains(t, text, "triage now")
		assert.Contains(t, text, `"kind": "priority"`)
	})

	t.Run("explains which workflows ran on a conversation and why the others did not", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		wf := k.workflow()
		k.d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()
		conv := k.conversation("Charged twice.")
		number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		k.llm.Script(tools(call("explain_workflow_run", map[string]any{"conversation_number": atoi(t, number)})),
			reply("It ran."))

		k.ask(k.thread(""), "Why did this get a note?").AssertOK()

		back := k.lastToolResult()
		assert.Contains(t, back, `"Triage" (`+wf+`): started`)
		assert.Contains(t, back, `of "Triage" (`+wf+`) · started`)
	})
}

/*
liveSeries publishes a contact series that people on the pro plan
enter: it waits two days, then tags them. It returns its id.
*/
func (k *desk) liveSeries(name string) string {
	k.t.Helper()
	tag := k.d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Onboarded " + name}).AssertCreated().
		String("data.id")
	id := k.d.Owner.PostJSON("/api/v1/series", testkit.JSON{"name": name, "subject": "contact", "spec": testkit.JSON{
		"entry": testkit.JSON{
			"rules":   testkit.JSON{"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"}},
			"reentry": testkit.JSON{"kind": "once"},
		},
		"nodes": []testkit.JSON{
			{"id": "entry", "kind": "entry"},
			{"id": "wait", "kind": "wait", "settings": testkit.JSON{"mode": "duration", "amount": 2, "unit": "days"}},
			{"id": "tagged", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}},
		},
		"edges": []testkit.JSON{
			{"from": "entry", "path": "next", "to": "wait"}, {"from": "wait", "path": "next", "to": "tagged"},
		},
	}}).AssertCreated().String("data.id")
	k.d.Owner.PostJSON("/api/v1/series/"+id+"/publications", testkit.JSON{"note": "go"}).AssertOK()
	k.d.TC.DrainLater()
	return id
}

/*
TestOperatorSeriesJourneys covers Operator explaining what series did
with a person, and why someone never entered.
*/
func TestOperatorSeriesJourneys(t *testing.T) {
	t.Parallel()

	t.Run("walks a journey step by step and says what it waits for", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		srs := k.liveSeries("Onboarding")
		ada := k.d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"name": "Ada Okafor", "email": "ada@customer.test", "custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertCreated().String("data.id")
		for range 3 {
			k.d.TC.DrainLater()
		}
		k.llm.Script(tools(call("explain_series_journey", map[string]any{"email": "ada@customer.test"})),
			reply("She waits."))

		k.ask(k.thread(""), "Where is Ada in onboarding?").AssertOK()

		back := k.lastToolResult()
		assert.Contains(t, back, "contact_id "+ada)
		assert.Contains(t, back, `Series "Onboarding" · series_id `+srs)
		assert.Contains(t, back, "entered ")
		assert.Contains(t, back, "by rules")
		assert.Contains(t, back, "1. step entry (entry)")
		assert.Contains(t, back, "took path next → wait")
		assert.Contains(t, back, `Now at step wait (wait {"mode":"duration","unit":"days","amount":2})`)
		assert.Contains(t, back, "Waiting for a wait step's time to pass (wait kind timer) until ")
		assert.Contains(t, back, "Entry rules: the contact matches")
	})

	t.Run("says which entry rule keeps someone out", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		srs := k.liveSeries("Pro welcome")
		bo := k.d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"name": "Bo Chen", "custom_attributes": testkit.JSON{"plan": "free"},
		}).AssertCreated().String("data.id")
		k.d.TC.DrainLater()
		k.llm.Script(tools(call("explain_series_journey", map[string]any{"contact_id": bo, "series_id": srs})),
			reply("Bo is on the free plan."))

		k.ask(k.thread(""), "Why did Bo never get the pro welcome?").AssertOK()

		back := k.lastToolResult()
		assert.Contains(t, back, "Journeys: none. This person never entered this series.")
		assert.Contains(t, back, "Entry rules: the contact does not match")
		assert.Contains(t, back, `- contact.custom.plan eq "pro": false · the contact has "free"`)
	})

	t.Run("refuses a call that names nobody", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(tools(call("explain_series_journey", map[string]any{})), reply("Who?"))

		k.ask(k.thread(""), "Explain the series").AssertOK()

		assert.Contains(t, k.lastToolResult(), "name the person by contact_id or email")
	})

	t.Run("tells Operator which step is selected in the series builder", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		srs := k.liveSeries("Selected")
		thread := k.thread("")
		k.llm.Script(reply("It waits two days."))

		k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body": "What does this step do?", "viewing": testkit.JSON{"page": "series", "id": srs, "step_id": "wait"},
		}).AssertOK()

		sent := k.llm.Sent()
		asked := sent[0].Messages[len(sent[0].Messages)-1]
		note := asked.Parts[len(asked.Parts)-2].Text
		assert.Contains(t, note, "The series builder is open on this series.")
		assert.Contains(t, note, `Selected step: wait · wait · settings {"mode":"duration","unit":"days","amount":2}`)
		assert.Contains(t, note, "When the teammate says this step, they mean step wait.")
	})
}

/*
TestOperatorSeries covers Operator editing and drafting series.
*/
func TestOperatorSeries(t *testing.T) {
	t.Parallel()

	t.Run("puts a new step on a path, applies it to the working copy and undoes it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		srs := k.d.Owner.PostJSON("/api/v1/series", testkit.JSON{"name": "Onboarding", "subject": "contact"}).
			AssertCreated().String("data.id")
		k.llm.Script(tools(call("propose_series_edit", map[string]any{
			"suggestion_title": "Welcome email", "reason": "New people need a hello.", "series_id": srs,
			"edits": []map[string]any{{
				"action": "add_step", "from_step_id": "entry",
				"step": map[string]any{"kind": "email", "settings": map[string]any{
					"subject": "Welcome", "body": "Hi {{contact.first_name}}, welcome aboard.",
				}},
			}},
		})), reply("Suggested."))

		res := k.ask(k.thread(""), "Send a welcome email first").AssertOK()
		res.AssertJSONPath("data.changesets.0.operations.0.kind", "series.edit_draft").
			AssertJSONPath("data.changesets.0.operations.0.series_id", srs)
		assert.Contains(t, k.lastToolResult(), "path next → email-1")

		id := res.String("data.changesets.0.id")
		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		k.d.Owner.GetJSON("/api/v1/series/"+srs).AssertOK().AssertJSONPath("data.spec.nodes.#", float64(2)).
			AssertJSONPath("data.spec.edges.0.to", "email-1")

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "reverted")
		k.d.Owner.GetJSON("/api/v1/series/"+srs).AssertOK().AssertJSONPath("data.spec.nodes.#", float64(1))
	})

	t.Run("refuses a message without a body, and drafts a new series once it reads right", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		spec := func(body string) map[string]any {
			return map[string]any{
				"nodes": []map[string]any{
					{"id": "entry", "kind": "entry"},
					{"id": "hello", "kind": "email", "settings": map[string]any{"subject": "Hi", "body": body}},
				},
				"edges": []map[string]any{{"from": "entry", "path": "next", "to": "hello"}},
			}
		}
		k.llm.Script(tools(call("propose_new_series", map[string]any{
			"suggestion_title": "Welcome series", "reason": "Say hello.", "name": "Welcome", "subject": "contact",
			"spec": spec(""),
		})), reply("Could not."))
		k.ask(k.thread(""), "Draft a welcome series").AssertOK().AssertJSONPath("data.changesets.#", float64(0))
		assert.Contains(t, k.lastToolResult(), "message_body_missing: Write the message. (step hello, setting body)")

		k.llm.Script(tools(call("propose_new_series", map[string]any{
			"suggestion_title": "Welcome series", "reason": "Say hello.", "name": "Welcome", "subject": "contact",
			"spec": spec("Welcome aboard."),
		})), reply("Drafted."))
		res := k.ask(k.thread(""), "Draft a welcome series").AssertOK()
		applied := k.d.Owner.PostJSON("/api/v1/operator/changesets/"+res.String("data.changesets.0.id")+
			"/applications", testkit.JSON{}).AssertOK()
		created := applied.String("data.operations.0.created_id")
		k.d.Owner.GetJSON("/api/v1/series/"+created).AssertOK().AssertJSONPath("data.status", "draft").
			AssertJSONPath("data.name", "Welcome")
	})
}
