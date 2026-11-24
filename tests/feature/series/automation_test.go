package series_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
withModel points the workspace's classifier at the scripted model.
*/
func withModel(t *testing.T, d desk, llm *fake.LLM) {
	t.Helper()

	d.tc.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return llm, nil
	})

	d.tc.InWorkspace(func(ctx context.Context) error {
		q, ws, now := models.New(d.tc.App.DB), d.tc.Workspace.Workspace.ID, d.tc.App.Clock.Now()
		key, err := q.CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: ws, Name: "fake", Provider: "fake", AuthType: "none", Status: "valid",
			Models: json.RawMessage("[]"), ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
			Source: "system", Now: now,
		})
		require.NoError(t, err)

		if _, err := q.FindDefaultAgent(ctx, ws); err != nil {
			_, err = q.CreateDefaultAgent(ctx, models.CreateDefaultAgentParams{WorkspaceID: ws, Now: now})
			require.NoError(t, err)
		}

		_, err = d.tc.App.DB.Exec(ctx, `INSERT INTO workspace_ai_settings
			(workspace_id, classifier_key_id, classifier_model) VALUES ($1, $2, 'sorter') ON CONFLICT (workspace_id)
			DO UPDATE SET classifier_key_id = $2, classifier_model = 'sorter'`, ws, key.ID)

		return err
	})
}

func TestSeriesAutomationSteps(t *testing.T) {
	t.Parallel()

	t.Run("runs a workspace action and extracts values onto the person", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		api := httpclienttest.New(d.tc.App.HTTP).PreventStrayRequests().
			Stub("GET https://crm.example.com/v1/plans*", httpclient.Response{Body: []byte(`{"data":{"seats":12}}`)})
		withModel(t, d, fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"fit":"team"}`}))

		conn := d.owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "CRM", "kind": "http", "base_url": "https://crm.example.com", "auth_type": "bearer",
			"token": "sealed-token-9012",
		}).AssertCreated().String("data.id")
		action := d.owner.PostJSON("/api/v1/ai/actions", testkit.JSON{
			"connection_id": conn, "name": "lookup_seats", "title": "Look up seats", "method": "GET",
			"description": "Look up seats by email.", "path_template": "/v1/plans",
			"query": testkit.JSON{"email": "{input.email}"},
			"input_schema": testkit.JSON{
				"type": "object", "required": []string{"email"},
				"properties": testkit.JSON{"email": testkit.JSON{"type": "string"}},
			},
			"response_fields": []testkit.JSON{{"name": "seats", "path": "data.seats"}},
		}).AssertCreated().String("data.id")
		d.owner.PostJSON("/api/v1/ai/actions/"+action+"/publications", testkit.JSON{}).AssertCreated()

		id := d.series("Sizing", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "seats", "kind": "run_action", "settings": testkit.JSON{
					"action_id": action,
					"inputs":    []testkit.JSON{{"name": "email", "value": "{{ contact.email }}"}},
					"save":      []testkit.JSON{{"output": "seats", "attribute": "seats"}},
				}},
				{"id": "fit", "kind": "extract", "settings": testkit.JSON{
					"prompt": "How big is their team?",
					"fields": []testkit.JSON{{"key": "fit", "type": "choice", "options": []string{"solo", "team"}}},
				}},
			},
			"edges": []testkit.JSON{edge("entry", "", "seats"), edge("seats", "", "fit")},
		})
		d.publish(id)

		ada := unique("ada")
		contact := d.contact(ada, "Ada Okafor", testkit.JSON{"plan": "pro"})
		d.drain()

		d.journeys(id).AssertJSONPath("data.0.status", "finished")

		sent := api.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, ada, sent[0].URL.Query().Get("email"))

		person := d.owner.GetJSON("/api/v1/contacts/" + contact).AssertOK()
		person.AssertJSONPath("data.custom_attributes.seats", float64(12))
		person.AssertJSONPath("data.custom_attributes.fit", "team")
	})
}
