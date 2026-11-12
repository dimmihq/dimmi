package workflows_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
publish creates a workflow from the definition, publishes it and
returns its id.
*/
func publish(t *testing.T, d tests.Desk, name string, spec testkit.JSON) string {
	t.Helper()

	id := create(t, d, testkit.JSON{"name": name, "spec": spec})
	d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()

	return id
}

/*
classifyingModel points the workspace's classifier at the scripted
model, with the agent itself left off.
*/
func classifyingModel(t *testing.T, d tests.Desk, llm *fake.LLM) {
	t.Helper()

	d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return llm, nil
	})

	d.TC.InWorkspace(func(ctx context.Context) error {
		q, ws, now := models.New(d.TC.App.DB), d.TC.Workspace.Workspace.ID, d.TC.App.Clock.Now()
		key, err := q.CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: ws, Name: "fake", Provider: "fake", AuthType: "none", Status: "valid",
			Models: json.RawMessage("[]"), ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
			Source: "system", Now: now,
		})
		require.NoError(t, err)

		agent, err := q.FindAgent(ctx, ws)
		if err != nil {
			agent, err = q.CreateAgent(ctx, models.CreateAgentParams{WorkspaceID: ws, Now: now})
			require.NoError(t, err)
		}

		_, err = d.TC.App.DB.Exec(ctx, `UPDATE ai_agents SET classifier_key_id = $1, classifier_model = 'sorter'
			WHERE id = $2`, key.ID, agent.ID)

		return err
	})
}

func TestCallSteps(t *testing.T) {
	t.Parallel()

	t.Run("tells the customer when the team replies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Reply time", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_reply01", "reply_time",
				testkit.JSON{"text": "Thanks {{contact.first_name}}! We reply {{reply_time}}."})},
		})

		got := bodies(t, d, conversation(t, d, "ada", "Hello"))
		require.Len(t, got, 2)
		assert.Regexp(t, `^Thanks ada! We reply (in a few minutes|within an hour|in a few hours|within a day|`+
			`later today after|tomorrow after|on [A-Z]).*\.$`, got[1])
	})

	t.Run("runs a reusable workflow and goes on with what it left", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "vip"}).AssertCreated().String("data.id")
		reusable := publish(t, d, "Ask plan", testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{
				step("s_tag0001", "tag", testkit.JSON{"tag_ids": []string{tag}}),
				step("s_hello01", "message", testkit.JSON{"text": "From the shared workflow"}),
			},
		})
		caller := publish(t, d, "Caller", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_call0001", "run_workflow", testkit.JSON{"workflow_id": reusable}),
				step("s_after001", "message", testkit.JSON{"text": "Back in the caller"}),
			},
		})

		conv := conversation(t, d, "bo", "Hello")
		got := bodies(t, d, conv)
		assert.Equal(t, []string{"Hello", "From the shared workflow", "Back in the caller"}, got)
		d.Owner.GetJSON(conv).AssertOK().AssertJSONPath("data.tags.0.id", tag)

		var runs struct {
			Data []struct {
				Status string `json:"status"`
				Steps  []struct {
					Kind    string         `json:"kind"`
					Outcome string         `json:"outcome"`
					Detail  map[string]any `json:"detail"`
				} `json:"steps"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/workflows/" + caller + "/runs").AssertOK().Decode(&runs)
		require.Len(t, runs.Data, 1)
		assert.Equal(t, "completed", runs.Data[0].Status)
		require.Len(t, runs.Data[0].Steps, 2)
		for _, s := range runs.Data[0].Steps {
			if s.Kind == "run_workflow" {
				assert.Equal(t, "completed", s.Outcome)
				assert.Equal(t, "completed", s.Detail["child_status"])
			}
		}
		d.Owner.GetJSON("/api/v1/workflows/"+reusable+"/runs").AssertOK().
			AssertJSONPath("data.0.trigger", "reusable").AssertJSONPath("data.0.status", "completed")
	})

	t.Run("refuses calls that lead back to the caller", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		inner := publish(t, d, "Inner", testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{step("s_prio001", "priority", testkit.JSON{"value": "high"})},
		})
		outer := publish(t, d, "Outer", testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{step("s_call0001", "run_workflow", testkit.JSON{"workflow_id": inner})},
		})

		d.Owner.PatchJSON("/api/v1/workflows/"+inner, testkit.JSON{"spec": testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{step("s_call0002", "run_workflow", testkit.JSON{"workflow_id": outer})},
		}}).AssertOK()
		d.Owner.PostJSON("/api/v1/workflows/"+inner+"/publications", nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("errors.0.code", "call_cycle")
	})

	t.Run("calls the customer's system through a connection and saves the answer", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		api := httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("POST https://crm.example.com/v1/lookup*", httpclient.Response{Body: []byte(`{"data":{"plan":"pro"}}`)})
		conn := d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "CRM", "kind": "http", "base_url": "https://crm.example.com", "auth_type": "bearer",
			"token": "sealed-token-1234",
		}).AssertCreated().String("data.id")

		live(t, d, "Lookup", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_http0001", "http_request", testkit.JSON{
					"connection_id": conn, "method": "POST", "url": "/v1/lookup?email={{contact.email}}",
					"body": `{"name":"{{contact.name}}"}`,
					"save": []testkit.JSON{{"path": "data.plan", "field": "run.vars.plan"}},
				}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
					step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
				}}),
				step("s_plan0001", "message", testkit.JSON{"text": "Your plan is {{run.vars.plan}}"}),
			},
		})

		id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.InboxID(), "author": "contact", "subject": "Help", "body": "Hello",
			"contact": testkit.JSON{"email": "cy+x@example.com", "name": `Cy "x"`},
		}).AssertCreated().String("data.id")
		d.TC.Drain()

		got := bodies(t, d, "/api/v1/conversations/"+id)
		assert.Equal(t, "Your plan is pro", got[len(got)-1])

		sent := api.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, "Bearer sealed-token-1234", sent[0].Header.Get("Authorization"))
		assert.Equal(t, "cy+x@example.com", sent[0].URL.Query().Get("email"))
		assert.JSONEq(t, `{"name":"Cy \"x\""}`, string(sent[0].Body))
	})

	t.Run("takes the failure path when the system answers with an error", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests().
			Stub("GET https://api.example.com/*", httpclient.Response{Status: 503})
		live(t, d, "Lookup", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_http0001", "http_request", testkit.JSON{
				"method": "GET", "url": "https://api.example.com/people/{{contact.email}}", "timeout": "2s",
			}, testkit.JSON{"id": "p_failed01", "outcome": "failure", "steps": []testkit.JSON{
				step("s_sorry001", "message", testkit.JSON{"text": "Lookup failed"}),
			}})},
		})

		got := bodies(t, d, conversation(t, d, "di", "Hello"))
		assert.Equal(t, "Lookup failed", got[len(got)-1])
	})

	sorter := func(id string) testkit.JSON {
		return step(id, "classify", testkit.JSON{"save_to": "run.vars.topic"},
			testkit.JSON{
				"id": "p_billing1", "label": "Billing", "description": "Invoices and refunds",
				"steps": []testkit.JSON{
					step("s_bill0001", "message", testkit.JSON{"text": "Billing: {{run.vars.topic}}"}),
				},
			},
			testkit.JSON{"id": "p_bugs0001", "label": "Bugs", "steps": []testkit.JSON{}},
			testkit.JSON{"id": "p_none0001", "outcome": "none", "steps": []testkit.JSON{
				step("s_none0001", "message", testkit.JSON{"text": "Not sorted"}),
			}})
	}

	t.Run("sorts a conversation into a category with the workspace's model", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		llm := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{
			Text: `{"category":"p_billing1","confidence":"high"}`,
		})
		classifyingModel(t, d, llm)
		live(t, d, "Sort", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{sorter("s_sort0001")},
		})

		got := bodies(t, d, conversation(t, d, "ed", "I was charged twice, please refund"))
		assert.Equal(t, "Billing: Billing", got[len(got)-1])

		sent := llm.Sent()
		require.Len(t, sent, 1)
		assert.True(t, strings.Contains(sent[0].Messages[0].Parts[0].Text, "charged twice"))
	})

	t.Run("takes the none path when the workspace has no model", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Sort", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{sorter("s_sort0001")},
		})

		got := bodies(t, d, conversation(t, d, "fi", "Something odd"))
		assert.Equal(t, "Not sorted", got[len(got)-1])
	})
}
