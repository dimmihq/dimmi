package ai_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
managedDesk is a desk on an install that offers dimmi's own models.
*/
func managedDesk(t *testing.T) tests.Desk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv("DIMMI_AI__MANAGED__ANTHROPIC__API_KEY=sk-ant-managed"))
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
}
