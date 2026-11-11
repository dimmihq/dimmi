package ai_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const (
	basetenCompletions = "POST https://inference.baseten.co/v1/chat/completions"
	engyCompletions    = "POST https://api.engy.ai/v1/chat/completions"
	basetenKeyEnv      = "DIMMI_AI__MANAGED__OPENWEIGHT__BASETEN__API_KEY=bt-managed"
	engyKeyEnv         = "DIMMI_AI__MANAGED__OPENWEIGHT__ENGY__API_KEY=sk-engy-managed"
)

/*
openWeightReply is a finished completion from an open-weight host.
*/
func openWeightReply(text string) string {
	return `{"id":"c1","object":"chat.completion","created":1,"model":"glm-5.3-flash",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"` + text + `"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`
}

/*
askBuiltIn sends one GLM Flash question through the built-in
connection of an install with the given environment, and returns the
answer, the hosts the requests reached and the error.
*/
func askBuiltIn(t *testing.T, stub func(*httpclienttest.Fake), env ...string) (string, []string, error) {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(env...))
	fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
	stub(fake)
	key := models.AiProviderKey{
		ID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID, Provider: string(enums.AIProviderDimmi),
	}
	drv, err := tc.App.LLM.ForKey(t.Context(), key)
	require.NoError(t, err)

	req := hello()
	req.Model = "glm-5.3-flash"
	res, err := drv.Chat(t.Context(), req)

	var hosts []string
	for _, sent := range fake.Sent() {
		hosts = append(hosts, sent.URL.Host)
	}
	if err != nil {
		return "", hosts, err
	}
	return res.Text, hosts, nil
}

func TestManagedOpenWeightHosts(t *testing.T) {
	t.Parallel()

	t.Run("answers from baseten when both hosts are set", func(t *testing.T) {
		t.Parallel()

		text, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusOK, openWeightReply("from baseten")))
		}, basetenKeyEnv, engyKeyEnv)

		require.NoError(t, err)
		assert.Equal(t, "from baseten", text)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts)
	})

	t.Run("answers from engy when baseten has no credit left", func(t *testing.T) {
		t.Parallel()

		text, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusPaymentRequired,
				`{"error":{"type":"insufficient_quota","code":"insufficient_quota","message":"no credit"}}`))
			f.Stub(engyCompletions, jsonReply(fhttp.StatusOK, openWeightReply("from engy")))
		}, basetenKeyEnv, engyKeyEnv)

		require.NoError(t, err)
		assert.Equal(t, "from engy", text)
		assert.Equal(t, []string{"inference.baseten.co", "api.engy.ai"}, hosts)
	})

	t.Run("answers from engy when baseten is failing", func(t *testing.T) {
		t.Parallel()

		text, _, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusBadGateway, `{"error":{"message":"bad gateway"}}`))
			f.Stub(engyCompletions, jsonReply(fhttp.StatusOK, openWeightReply("from engy")))
		}, basetenKeyEnv, engyKeyEnv)

		require.NoError(t, err)
		assert.Equal(t, "from engy", text)
	})

	t.Run("keeps a refused request on baseten", func(t *testing.T) {
		t.Parallel()

		_, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusBadRequest,
				`{"error":{"type":"invalid_request_error","code":"invalid_request_error","message":"bad"}}`))
		}, basetenKeyEnv, engyKeyEnv)

		llmErr, ok := errors.AsType[*contracts.LLMError](err)
		require.True(t, ok, "error %v is not an LLMError", err)
		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmErr.Kind)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts)
	})

	t.Run("uses engy alone when only engy is set", func(t *testing.T) {
		t.Parallel()

		text, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(engyCompletions, jsonReply(fhttp.StatusOK, openWeightReply("from engy")))
		}, engyKeyEnv)

		require.NoError(t, err)
		assert.Equal(t, "from engy", text)
		assert.Equal(t, []string{"api.engy.ai"}, hosts)
	})

	t.Run("uses baseten alone when only baseten is set", func(t *testing.T) {
		t.Parallel()

		_, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusPaymentRequired,
				`{"error":{"type":"insufficient_quota","code":"insufficient_quota","message":"no credit"}}`))
		}, basetenKeyEnv)

		require.Error(t, err)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts)
	})

	t.Run("keeps the older single key working on the host it names", func(t *testing.T) {
		t.Parallel()

		text, hosts, err := askBuiltIn(t, func(f *httpclienttest.Fake) {
			f.Stub(basetenCompletions, jsonReply(fhttp.StatusOK, openWeightReply("from baseten")))
		}, "DIMMI_AI__MANAGED__OPENWEIGHT__API_KEY=bt-old", "DIMMI_AI__MANAGED__OPENWEIGHT__PROVIDER=baseten")

		require.NoError(t, err)
		assert.Equal(t, "from baseten", text)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts)
	})

	t.Run("offers glm flash when only one open-weight host is set", func(t *testing.T) {
		t.Parallel()
		d := managedDesk(t, basetenKeyEnv)

		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK().AssertJSONPath("data.agent.answer_model", "glm-5.3-flash")
	})
}
