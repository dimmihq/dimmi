package workflows_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/tests"
)

/*
extractor is an extract step that looks for an order number and
whether the customer wants a refund, with a message after it and one
on its none path.
*/
func extractor(fallback any) testkit.JSON {
	refund := testkit.JSON{"key": "refund", "type": "boolean", "description": "Whether they ask for a refund"}
	if fallback != nil {
		refund["fallback"] = fallback
	}

	return step("s_extr0001", "extract", testkit.JSON{
		"instructions": "Find the order.",
		"fields": []testkit.JSON{
			{"key": "order", "label": "Order number", "type": "text", "save_to": "run.vars.order"},
			refund,
		},
	}, testkit.JSON{"id": "p_none0001", "outcome": "none", "steps": []testkit.JSON{
		step("s_none0001", "message", testkit.JSON{"text": "Which order is it?"}),
	}})
}

func TestAutomationSteps(t *testing.T) {
	t.Parallel()

	t.Run("extracts typed values into run variables", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"order":"A-1234","refund":true}`})
		classifyingModel(t, d, llm)
		live(t, d, "Extract", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{
				extractor(nil),
				step("s_said0001", "message", testkit.JSON{
					"text": "Order {{run.vars.order}}, refund {{run.vars.refund}}",
				}),
			},
		})

		got := bodies(t, d, conversation(t, d, "ana", "My order A-1234 came broken, refund please"))
		assert.Equal(t, "Order A-1234, refund true", got[len(got)-1])

		sent := llm.Sent()
		require.Len(t, sent, 1)
		assert.Contains(t, sent[0].Messages[0].Parts[0].Text, "Customer: My order A-1234")
		require.NotNil(t, sent[0].Schema)
		assert.Contains(t, string(sent[0].Schema.Schema), `"refund"`)
	})

	t.Run("takes the none path for an unknown value, and a fallback for one it has", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"order":null,"refund":null}`})
		classifyingModel(t, d, llm)
		live(t, d, "Extract", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{extractor(false)},
		})

		got := bodies(t, d, conversation(t, d, "bea", "Hello there"))
		assert.Equal(t, "Which order is it?", got[len(got)-1])
	})

	t.Run("takes the none path when the workspace has no model", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Extract", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{extractor(nil)},
		})

		got := bodies(t, d, conversation(t, d, "cal", "Order A-1"))
		assert.Equal(t, "Which order is it?", got[len(got)-1])
	})

	t.Run("refuses a fallback of the wrong type", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Extract", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{extractor("yes")},
		}})
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).AssertStatus(422).
			AssertJSONPath("errors.0.code", "invalid_setting")
	})

	t.Run("runs a workspace action and saves its outputs", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		api := httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("GET https://crm.example.com/v1/plans*", httpclient.Response{
				Body: []byte(`{"data":{"plan":"pro","seats":3}}`),
			})
		action := publishedAction(t, d)

		live(t, d, "Lookup", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_act00001", "run_action", testkit.JSON{
					"action_id": action,
					"inputs":    []testkit.JSON{{"name": "email", "value": "{{contact.email}}"}},
					"save":      []testkit.JSON{{"output": "plan", "field": "run.vars.plan"}},
				}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
					step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
				}}),
				step("s_plan0001", "message", testkit.JSON{"text": "Your plan is {{run.vars.plan}}"}),
			},
		})

		got := bodies(t, d, conversation(t, d, "dee", "Hello"))
		assert.Equal(t, "Your plan is pro", got[len(got)-1])

		sent := api.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, "dee@example.com", sent[0].URL.Query().Get("email"))
		assert.Equal(t, "Bearer sealed-token-5678", sent[0].Header.Get("Authorization"))

		d.Owner.GetJSON("/api/v1/ai/action-calls?filter[action_id]="+action).AssertOK().
			AssertJSONPath("data.0.state", "succeeded")
	})

	t.Run("takes the failure path when the action fails", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("GET https://crm.example.com/v1/plans*", httpclient.Response{Status: 500, Body: []byte(`{}`)})
		action := publishedAction(t, d)

		live(t, d, "Lookup", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_act00001", "run_action", testkit.JSON{
					"action_id": action, "inputs": []testkit.JSON{{"name": "email", "value": "{{contact.email}}"}},
				}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
					step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
				}}),
			},
		})

		got := bodies(t, d, conversation(t, d, "eve", "Hello"))
		assert.Equal(t, "Lookup failed", got[len(got)-1])
	})
}

/*
publishedAction connects a CRM and publishes an action that looks up a
plan by email, returning the action's id.
*/
func publishedAction(t *testing.T, d tests.Desk) string {
	t.Helper()

	conn := d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
		"name": "CRM", "kind": "http", "base_url": "https://crm.example.com", "auth_type": "bearer",
		"token": "sealed-token-5678",
	}).AssertCreated().String("data.id")

	id := d.Owner.PostJSON("/api/v1/ai/actions", testkit.JSON{
		"connection_id": conn, "name": "lookup_plan", "title": "Look up plan", "method": "GET",
		"description": "Look up a customer's plan by email.", "path_template": "/v1/plans",
		"query": testkit.JSON{"email": "{input.email}"},
		"input_schema": testkit.JSON{
			"type": "object", "required": []string{"email"},
			"properties": testkit.JSON{"email": testkit.JSON{"type": "string"}},
		},
		"response_fields": []testkit.JSON{{"name": "plan", "path": "data.plan"}},
	}).AssertCreated().String("data.id")
	d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertCreated()

	return id
}

func TestAIConversationAttributes(t *testing.T) {
	t.Parallel()

	t.Run("detects an attribute, keeps it on the conversation and starts workflows on it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"intent":"refund","_reasons":{"intent":"The customer asks for their money back."}}`})
		classifyingModel(t, d, llm)

		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", testkit.JSON{
			"key": "intent", "label": "Intent", "description": "What the customer wants",
			"data_type": "choice", "options": []string{"refund", "question"},
		}).AssertCreated().AssertJSONPath("data.key", "intent").AssertJSONPath("data.status", "active")

		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "refunds"}).AssertCreated().String("data.id")
		live(t, d, "Refunds", testkit.JSON{
			"schema": 1, "trigger": "conversation.ai_attribute_changed",
			"audience": testkit.JSON{
				"rules": testkit.JSON{"field": "conversation.ai.intent", "op": "eq", "value": "refund"},
			},
			"steps": []testkit.JSON{step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}})},
		})

		conv := conversation(t, d, "fay", "I want my money back")
		d.TC.DrainLater()
		d.TC.Drain()

		d.Owner.GetJSON(conv+"/ai-attributes").AssertOK().
			AssertJSONPath("data.values.0.attribute.key", "intent").AssertJSONPath("data.values.0.value", "refund")
		d.Owner.GetJSON(conv+"/attributes").AssertOK().
			AssertJSONPath("data.values.0.set_by.by", "ai").
			AssertJSONPath("data.values.0.set_by.reason", "The customer asks for their money back.")
		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.tags.0.id", tag)

		sent := llm.Sent()
		require.NotEmpty(t, sent)
		assert.True(t, strings.Contains(sent[0].Messages[0].Parts[0].Text, "money back"))
	})

	t.Run("refuses a choice without choices and a key taken", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", testkit.JSON{
			"key": "intent", "label": "Intent", "data_type": "choice",
		}).AssertStatus(422).AssertJSONPath("code", "options_invalid")

		body := testkit.JSON{"key": "order", "label": "Order", "data_type": "text"}
		id := d.Owner.PostJSON("/api/v1/ai/conversation-attributes", body).AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", body).AssertStatus(409)

		d.Owner.PatchJSON("/api/v1/ai/conversation-attributes/"+id, testkit.JSON{"status": "archived"}).AssertOK().
			AssertJSONPath("data.status", "archived")
		d.Owner.GetJSON("/api/v1/ai/conversation-attributes").AssertOK().AssertJSONPath("data.0.key", "order")
	})

	t.Run("never replaces a value a teammate set, and starts workflows on any change", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"intent":"refund","_reasons":{"intent":"The customer asks for their money back."}}`})
		classifyingModel(t, d, llm)

		d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "intent", "label": "Intent", "description": "What the customer wants",
			"data_type": "choice", "options": []string{"refund", "question"},
		}).AssertCreated().AssertJSONPath("data.filled_by", "both")

		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "questions"}).AssertCreated().String("data.id")
		live(t, d, "Questions", testkit.JSON{
			"schema": 1, "trigger": "conversation.attribute_changed",
			"audience": testkit.JSON{"rules": testkit.JSON{"all": []testkit.JSON{
				{"field": "attribute.key", "op": "eq", "value": "intent"},
				{"field": "attribute.by", "op": "eq", "value": "member"},
				{"field": "conversation.ai.intent", "op": "eq", "value": "question"},
			}}},
			"steps": []testkit.JSON{step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}})},
		})

		conv := conversation(t, d, "gil", "I want my money back")
		d.Owner.PatchJSON(conv+"/attributes", testkit.JSON{"values": testkit.JSON{"intent": "question"}}).
			AssertOK()
		d.TC.DrainLater()
		d.TC.Drain()

		d.Owner.GetJSON(conv+"/attributes").AssertOK().
			AssertJSONPath("data.values.0.value", "question").AssertJSONPath("data.values.0.set_by.by", "member")
		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.tags.0.id", tag)
	})

	t.Run("a workflow step sets attributes from variables, and fails on a value that does not fit", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing", "Bug"},
			"filled_by": "teammates",
		}).AssertCreated()
		d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "seats", "label": "Seats", "data_type": "number", "filled_by": "teammates",
		}).AssertCreated()

		live(t, d, "Triage", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_set00001", "set_conversation_attributes", testkit.JSON{"attributes": []testkit.JSON{
					{"key": "issue", "value": "Billing"}, {"key": "seats", "value": "12"},
				}}),
				step("s_set00002", "set_conversation_attributes", testkit.JSON{"attributes": []testkit.JSON{
					{"key": "seats", "value": "many"},
				}}),
			},
		})

		conv := conversation(t, d, "hal", "Invoice question")
		d.TC.DrainLater()
		d.TC.Drain()

		res := d.Owner.GetJSON(conv + "/attributes").AssertOK()
		res.AssertJSONPath("data.values.0.attribute.key", "issue").AssertJSONPath("data.values.0.value", "Billing").
			AssertJSONPath("data.values.0.set_by.by", "workflow").
			AssertJSONPath("data.values.0.set_by.name", "Workflow: Triage")
		res.AssertJSONPath("data.values.1.attribute.key", "seats").AssertJSONPath("data.values.1.value", 12.0)
	})

	t.Run("deletes an attribute only while no conversation, workflow or view uses it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		used := d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing"},
		}).AssertCreated().String("data.id")
		held := d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "plan", "label": "Plan", "data_type": "text",
		}).AssertCreated().String("data.id")
		free := d.Owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "issue_extra", "label": "Extra", "data_type": "text",
		}).AssertCreated().String("data.id")

		create(t, d, testkit.JSON{"name": "Billing triage", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"audience": testkit.JSON{
				"rules": testkit.JSON{"field": "conversation.ai.issue", "op": "eq", "value": "Billing"},
			},
			"steps": []testkit.JSON{step("s_note0001", "note", testkit.JSON{"text": "Billing"})},
		}})

		conv := conversation(t, d, "ida", "Hello")
		d.Owner.PatchJSON(conv+"/attributes", testkit.JSON{"values": testkit.JSON{"plan": "Pro"}}).AssertOK()

		d.Owner.GetJSON("/api/v1/conversation-attributes/"+used+"/usage").AssertOK().
			AssertJSONPath("data.deletable", false).AssertJSONPath("data.workflows.0.name", "Billing triage")
		d.Owner.Delete("/api/v1/conversation-attributes/" + used).AssertStatus(409).
			AssertJSONPath("code", "attribute_in_use").AssertJSONPath("workflows.0", "Billing triage")

		d.Owner.GetJSON("/api/v1/conversation-attributes/"+held+"/usage").AssertOK().
			AssertJSONPath("data.conversations", 1.0)
		d.Owner.Delete("/api/v1/conversation-attributes/" + held).AssertStatus(409)

		d.Owner.GetJSON("/api/v1/conversation-attributes/"+free+"/usage").AssertOK().
			AssertJSONPath("data.deletable", true)
		d.Owner.Delete("/api/v1/conversation-attributes/" + free).AssertStatus(204)
		d.Owner.GetJSON("/api/v1/conversation-attributes").AssertOK().AssertJSONPath("data.#", 2)
	})
}
