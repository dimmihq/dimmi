package ai_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
managedDesk is a desk on an install that offers dimmi's own models.
*/
func managedDesk(t *testing.T, env ...string) tests.Desk {
	t.Helper()

	if len(env) == 0 {
		env = []string{"DIMMI_AI__MANAGED__ANTHROPIC__API_KEY=sk-ant-managed"}
	}
	tc := tests.New(t, tests.WithEnv(env...))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	return tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
}

/*
managedIDs are the model ids the built-in connection offers today.
*/
func managedIDs() []string {
	var out []string
	for _, e := range aimodels.Managed() {
		out = append(out, e.ID)
	}
	return out
}

func TestManagedConnection(t *testing.T) {
	t.Parallel()

	t.Run("names the built-in connection dimmi and offers today's models", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t)

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.keys.#(provider==\"dimmi\").name", "dimmi")
		res.AssertJSONPath("data.keys.#(provider==\"dimmi\").models", managedIDs())
	})

	t.Run("brings an older built-in connection up to date", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t)
		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()
		_, err := d.TC.Operator().Exec(t.Context(), `
			UPDATE ai_provider_keys SET name = 'Dimmi (built-in)', models = '["claude-retired", "gpt-retired"]'
			WHERE workspace_id = $1 AND provider = 'dimmi'`, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.keys.#(provider==\"dimmi\").name", "dimmi")
		res.AssertJSONPath("data.keys.#(provider==\"dimmi\").models", managedIDs())
		res.AssertJSONPath("data.keys.#(provider==\"dimmi\").models_count", len(managedIDs()))
		res.AssertJSONPath("data.providers.#(id==\"dimmi\").name", "dimmi")
	})

	t.Run("has a fresh agent ready on the built-in connection with GPT-6 Luna", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t,
			"DIMMI_AI__MANAGED__ANTHROPIC__API_KEY=sk-ant-managed",
			"DIMMI_AI__MANAGED__OPENAI__API_KEY=sk-openai-managed")

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.agent.answer_key_id", res.String("data.keys.#(provider==\"dimmi\").id"))
		res.AssertJSONPath("data.agent.answer_model", "gpt-6-luna")
		res.AssertJSONPath("data.ready", true)
		res.AssertJSONPath("data.managed", true)
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"enabled": true}).AssertOK().
			AssertJSONPath("data.agent.enabled", true)
	})

	t.Run("answers with Claude Haiku when the built-in provider has only Anthropic", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t)

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.agent.answer_model", "claude-haiku-5-5")
		res.AssertJSONPath("data.ready", true)
	})

	t.Run("answers with GLM Flash when the built-in provider has only open-weight models", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t, "DIMMI_AI__MANAGED__OPENWEIGHT__API_KEY=sk-engy-managed")

		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK().AssertJSONPath("data.agent.answer_model", "glm-5.3-flash")
	})

	t.Run("offers the built-in provider as unavailable without built-in credentials", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.managed", false)
		res.AssertJSONPath("data.providers.#(id==\"dimmi\").available", false)
		res.AssertJSONPath("data.agent.answer_key_id", nil)
		res.AssertJSONPath("data.ready", false)
	})
}
