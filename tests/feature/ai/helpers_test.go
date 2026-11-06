package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
answering stands in for the Anthropic and OpenAI-compatible drivers
with a fake that lists the given models, so a key validates without
leaving the process.
*/
func answering(d tests.Desk, modelIDs ...string) {
	listed := make([]contracts.LLMModel, len(modelIDs))
	for i, id := range modelIDs {
		listed[i] = contracts.LLMModel{ID: id}
	}
	llm := fake.New(contracts.LLMProviderAnthropic).Models(listed...)

	for _, provider := range []enums.AIProvider{enums.AIProviderAnthropic, enums.AIProviderOpenAICompatible} {
		d.TC.App.LLM.Extend(provider, func(
			_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials,
			_ *httpclient.Client,
		) (contracts.LLM, error) {
			return llm, nil
		})
	}
}

/*
refusing stands in for the OpenAI-compatible driver with one whose server
refuses the key, with a raw message no response may repeat.
*/
func refusing(d tests.Desk) {
	d.TC.App.LLM.Extend(enums.AIProviderOpenAICompatible, func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return nil, &contracts.LLMError{
			Kind: contracts.LLMErrorAuth, HTTPStatus: 401,
			Message: "401 from https://internal.example.com/v1/models for key sk-ant-leak",
		}
	})
}

/*
createKey connects an OpenAI-compatible server that needs no key as
the owner and returns its public id.
*/
func createKey(t *testing.T, d tests.Desk, name string) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
		"name": name, "provider": "openai_compatible", "base_url": "https://llm.example.com/v1",
	}).AssertCreated().String("data.id")
}

/*
agentRow makes sure the workspace's agent exists and returns its id.
*/
func agentRow(t *testing.T, d tests.Desk) ids.UUID {
	t.Helper()

	d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

	var id ids.UUID
	err := d.TC.Operator().QueryRow(t.Context(),
		"SELECT id FROM ai_agents WHERE workspace_id = $1 AND deleted_at IS NULL",
		d.TC.Workspace.Workspace.ID).Scan(&id)
	require.NoError(t, err)

	return id
}

/*
run is one run to seed, with the step it took.
*/
type run struct {
	conversation *ids.UUID
	kind         string
	outcome      string
	cost         int64
	model        string
	at           time.Time
	payload      bool
}

/*
seedRun inserts a finished run and its one step as an operator, and
returns the run's id.
*/
func seedRun(t *testing.T, d tests.Desk, agent ids.UUID, r run) ids.UUID {
	t.Helper()

	if r.kind == "" {
		r.kind = "conversation"
	}

	var id ids.UUID
	err := d.TC.Operator().QueryRow(t.Context(), `
		INSERT INTO ai_runs (
			workspace_id, agent_id, kind, conversation_id, status, outcome, cost_micros,
			input_tokens, output_tokens, completed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'completed', $5, $6, 100, 20, $7, $7, $7)
		RETURNING id`,
		d.TC.Workspace.Workspace.ID, agent, r.kind, r.conversation, r.outcome, r.cost, r.at).Scan(&id)
	require.NoError(t, err)

	var request, response any
	if r.payload {
		request, response = json.RawMessage(`{"prompt":"secret prompt"}`), json.RawMessage(`{"text":"reply"}`)
	}
	_, err = d.TC.Operator().Exec(t.Context(), `
		INSERT INTO ai_run_steps (
			workspace_id, run_id, seq, kind, provider, model, status, request, response,
			input_tokens, output_tokens, cost_micros, created_at
		) VALUES ($1, $2, 1, 'generate', 'anthropic', $3, 'ok', $4, $5, 100, 20, $6, $7)`,
		d.TC.Workspace.Workspace.ID, id, r.model, request, response, r.cost, r.at)
	require.NoError(t, err)

	return id
}

/*
conversationOf starts a customer conversation and returns its public
and internal ids.
*/
func conversationOf(t *testing.T, d tests.Desk, name string) (string, ids.UUID) {
	t.Helper()

	public := d.Customer(t, name, "Help").String("data.id")
	id, err := d.TC.App.IDs.Decode(string(models.PrefixConversation), public)
	require.NoError(t, err)

	return public, id
}
