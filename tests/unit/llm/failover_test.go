package llm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/baseten"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/failover"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/config"
)

const (
	basetenURL = "POST https://inference.baseten.co/v1/chat/completions"
	engyURL    = "POST https://api.engy.ai/v1/chat/completions"
)

/*
openWeightHosts builds Baseten and Engy drivers over one fake, in the
order the built-in provider tries them.
*/
func openWeightHosts(t *testing.T) ([]contracts.LLM, *httpclienttest.Fake) {
	t.Helper()

	hc, fake := fakeHTTP(t)
	return []contracts.LLM{
		baseten.New(baseten.Options{APIKey: "bt-key", HTTP: hc.StdClient()}),
		engy.New(engy.Options{APIKey: "sk-engy", HTTP: hc.StdClient()}),
	}, fake
}

/*
completion is a finished chat completion with text and usage.
*/
func completion(text string) httpclient.Response {
	return jsonResponse(fhttp.StatusOK, `{"id":"c1","object":"chat.completion","created":1,"model":"glm-5.3-flash",`+
		`"choices":[{"index":0,"message":{"role":"assistant","content":"`+text+`"},"finish_reason":"stop"}],`+
		`"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`)
}

/*
hosts lists the host of each request the fake received.
*/
func hosts(fake *httpclienttest.Fake) []string {
	var out []string
	for _, req := range fake.Sent() {
		out = append(out, req.URL.Host)
	}
	return out
}

func TestFailoverDriver(t *testing.T) {
	t.Parallel()

	t.Run("answers from the first host when it serves", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, completion("from baseten"))
		drv := failover.New(failover.Options{Hosts: list})

		res, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		require.NoError(t, err)
		assert.Equal(t, "from baseten", res.Text)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts(fake))
		assert.Equal(t, contracts.LLMProviderBaseten, drv.Provider())
	})

	t.Run("moves to the next host when the first has no credit left", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusPaymentRequired, apiError("insufficient_quota", "no credit")))
		fake.Stub(engyURL, completion("from engy"))
		drv := failover.New(failover.Options{Hosts: list})

		res, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		require.NoError(t, err)
		assert.Equal(t, "from engy", res.Text)
		assert.Equal(t, int64(12), res.Usage.InputTokens)
		assert.Equal(t, int64(5), res.Usage.OutputTokens)
		assert.Equal(t, "zai-org/GLM-5.3-Flash", body(t, fake.Sent()[0])["model"])
		assert.Equal(t, "glm-5.3-flash", body(t, fake.Sent()[1])["model"])
		assert.Equal(t, "Bearer sk-engy", fake.Sent()[1].Header.Get("Authorization"))
	})

	t.Run("opens the stream on the next host when the first is failing", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusServiceUnavailable, apiError("server_error", "down")))
		fake.Stub(engyURL, sse("", chunk("ok", "stop"), "", "[DONE]"))
		drv := failover.New(failover.Options{Hosts: list})

		stream, err := drv.Stream(t.Context(), request("glm-5.3-flash"))
		require.NoError(t, err)

		assert.Equal(t, "ok", find(drain(t, stream), contracts.StreamEventTextDelta).Text)
		assert.Equal(t, []string{"inference.baseten.co", "api.engy.ai"}, hosts(fake))
	})

	t.Run("moves on when the first host throttles", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusTooManyRequests, apiError("rate_limit_exceeded", "slow down")))
		fake.Stub(engyURL, completion("from engy"))
		drv := failover.New(failover.Options{Hosts: list})

		res, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		require.NoError(t, err)
		assert.Equal(t, "from engy", res.Text)
	})

	t.Run("never resends a request the caller got wrong", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusBadRequest, apiError("invalid_request_error", "bad")))
		drv := failover.New(failover.Options{Hosts: list})

		_, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmError(t, err).Kind)
		assert.Equal(t, []string{"inference.baseten.co"}, hosts(fake))
	})

	t.Run("returns the last host's failure when none can serve", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusServiceUnavailable, apiError("server_error", "down")))
		fake.Stub(engyURL, jsonResponse(fhttp.StatusPaymentRequired, apiError("insufficient_quota", "no credit")))
		drv := failover.New(failover.Options{Hosts: list})

		_, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		assert.Equal(t, contracts.LLMErrorQuota, llmError(t, err).Kind)
		assert.Len(t, fake.Sent(), 2)
	})

	t.Run("skips a host whose breaker is open", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(basetenURL, jsonResponse(fhttp.StatusPaymentRequired, apiError("insufficient_quota", "no credit")))
		fake.Stub(engyURL, completion("from engy"))
		breakers := llmretry.NewBreakers(clock.System(), configBreaker())
		drv := failover.New(failover.Options{
			Hosts: list,
			BreakerFor: func(host contracts.LLMProvider, model string) *llmretry.Breaker {
				return breakers.For(string(host) + ":" + model)
			},
		})

		for range 3 {
			_, err := drv.Chat(t.Context(), request("glm-5.3-flash"))
			require.NoError(t, err)
		}

		assert.Equal(t, []string{"inference.baseten.co", "api.engy.ai", "api.engy.ai", "api.engy.ai"}, hosts(fake))
	})

	t.Run("hands back a lone host as it is", func(t *testing.T) {
		t.Parallel()
		list, fake := openWeightHosts(t)
		fake.Stub(engyURL, completion("from engy"))
		drv := failover.New(failover.Options{Hosts: list[1:]})

		res, err := drv.Chat(t.Context(), request("glm-5.3-flash"))

		require.NoError(t, err)
		assert.Equal(t, "from engy", res.Text)
		assert.Equal(t, contracts.LLMProviderEngy, drv.Provider())
		assert.Equal(t, list[1], drv)
	})

	t.Run("treats only host failures as a reason to move", func(t *testing.T) {
		t.Parallel()
		for kind, moves := range map[contracts.LLMErrorKind]bool{
			contracts.LLMErrorQuota: true, contracts.LLMErrorAuth: true, contracts.LLMErrorPermission: true,
			contracts.LLMErrorRateLimited: true, contracts.LLMErrorOverloaded: true, contracts.LLMErrorServer: true,
			contracts.LLMErrorTimeout: true, contracts.LLMErrorNetwork: true, contracts.LLMErrorModelNotFound: true,
			contracts.LLMErrorBlockedHost: true, contracts.LLMErrorInvalidRequest: false,
			contracts.LLMErrorContentFiltered: false, contracts.LLMErrorCanceled: false,
		} {
			assert.Equal(t, moves, failover.HostUnavailable(&contracts.LLMError{Kind: kind}), kind)
		}
		assert.False(t, failover.HostUnavailable(contracts.ErrSchemaUnsupported))
	})
}

/*
configBreaker opens a breaker after one failure for a minute.
*/
func configBreaker() config.AIBreakerConfig {
	return config.AIBreakerConfig{Failures: 1, Window: time.Minute, Open: time.Minute}
}
