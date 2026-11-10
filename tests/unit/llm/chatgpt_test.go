package llm_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/llm"
)

const (
	responsesURL = "POST https://chatgpt.com/backend-api/codex/responses"
	issuer       = "https://auth.openai.com"
	exchangeURL  = "https://auth.openai.com/oauth/token"
)

/*
tokens is a scripted token source that hands out a new bearer on
every renewal.
*/
type tokens struct {
	mu      sync.Mutex
	renewed int
}

func (s *tokens) Token(context.Context) (chatgpt.Token, error) {
	return chatgpt.Token{AccessToken: "access-0", AccountID: "acct_1"}, nil
}

func (s *tokens) Renew(context.Context) (chatgpt.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewed++
	return chatgpt.Token{AccessToken: "access-1", AccountID: "acct_1"}, nil
}

/*
toolTurns is a transcript with an earlier tool call and its result.
*/
func toolTurns(req contracts.ChatRequest) contracts.ChatRequest {
	req.Tools = []contracts.ToolSpec{{Name: "lookup", Description: "Finds", Schema: json.RawMessage(`{}`)}}
	req.Messages = append(req.Messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{
			{ID: "call_0", Name: "lookup", Arguments: json.RawMessage(`{"q":"a"}`)},
		}},
		contracts.ChatMessage{
			Role: contracts.ChatRoleTool, ToolCallID: "call_0", Parts: []contracts.ContentPart{{Text: "found"}},
		},
	)
	return req
}

func TestChatGPTDriver(t *testing.T) {
	t.Parallel()

	t.Run("sends the plan's headers and a body without output bounds or sampling", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(responsesURL, fixtures.Reply("chatgpt_tool.sse"))
		drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: &tokens{}})

		temp := 0.3
		req := toolTurns(request("gpt-6.1-sol"))
		req.Temperature, req.MaxOutputTokens = &temp, 500
		req.Schema = verdictSchema()
		req.Metadata = map[string]string{"session_id": "conv_1"}
		stream, err := drv.Stream(t.Context(), req)
		require.NoError(t, err)
		drain(t, stream)

		h := fake.Sent()[0].Header
		assert.Equal(t, "Bearer access-0", h.Get("Authorization"))
		assert.Equal(t, "acct_1", h.Get("chatgpt-account-id"))
		assert.Equal(t, "dimmi", h.Get("originator"))
		assert.Equal(t, "conv_1", h.Get("session-id"))
		assert.Equal(t, "responses=experimental", h.Get("OpenAI-Beta"))

		sent := body(t, fake.Sent()[0])
		assert.Equal(t, false, sent["store"])
		assert.Equal(t, true, sent["stream"])
		assert.Equal(t, "Be brief.", sent["instructions"])
		assert.NotContains(t, sent, "max_output_tokens")
		assert.NotContains(t, sent, "temperature")
		assert.Equal(t, "auto", sent["tool_choice"])
		format := object(object(sent["text"])["format"])
		assert.Equal(t, "json_schema", format["type"])
		assert.Equal(t, "verdict", format["name"])
		assert.Equal(t, true, format["strict"])
		assert.Equal(t, "object", object(format["schema"])["type"])
		input := list(sent["input"])
		require.Len(t, input, 3)
		assert.Equal(t, "function_call", object(input[1])["type"])
		assert.Equal(t, "call_0", object(input[1])["call_id"])
		assert.Equal(t, "function_call_output", object(input[2])["type"])
		assert.Equal(t, "call_0", object(input[2])["call_id"])
		assert.Equal(t, "Finds", object(list(sent["tools"])[0])["description"])
	})

	t.Run("streams a tool call with its announced id and name", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(responsesURL, fixtures.Reply("chatgpt_tool.sse"))
		drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: &tokens{}})

		res, err := drv.Chat(t.Context(), request("gpt-6.1-sol"))

		require.NoError(t, err)
		assert.Equal(t, "Let me check.", res.Text)
		require.Len(t, res.ToolCalls, 1)
		assert.Equal(t, contracts.ToolCall{
			ID: "call_1", Name: "lookup", Arguments: json.RawMessage(`{"q":"x"}`),
		}, res.ToolCalls[0])
		assert.Equal(t, contracts.StopReasonToolUse, res.StopReason)
		assert.Equal(t, contracts.LLMUsage{
			InputTokens: 60, CachedInputTokens: 40, OutputTokens: 10, ReasoningTokens: 3, Reported: true,
		}, res.Usage)
	})

	t.Run("ends an incomplete response on its reason", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(responsesURL, fixtures.Reply("chatgpt_incomplete.sse"))
		drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: &tokens{}})

		res, err := drv.Chat(t.Context(), request("gpt-6.1-sol"))

		require.NoError(t, err)
		assert.Equal(t, contracts.StopReasonMaxTokens, res.StopReason)
		assert.Equal(t, "Partial", res.Text)
	})

	t.Run("fails on a failed response, an error event or a cut stream", func(t *testing.T) {
		t.Parallel()
		for name, kind := range map[string]contracts.LLMErrorKind{
			"chatgpt_failed.sse": contracts.LLMErrorRateLimited,
			"chatgpt_error.sse":  contracts.LLMErrorServer,
			"chatgpt_cut.sse":    contracts.LLMErrorNetwork,
		} {
			hc, fake := fakeHTTP(t)
			fake.Stub(responsesURL, fixtures.Reply(name))
			drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: &tokens{}})

			_, err := drv.Chat(t.Context(), request("gpt-6.1-sol"))

			assert.Equal(t, kind, llmError(t, err).Kind, name)
		}
	})

	t.Run("renews a refused bearer once and tries again", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.StubSequence(responsesURL,
			jsonResponse(fhttp.StatusUnauthorized, apiError("token_expired", "expired")),
			fixtures.Reply("chatgpt_tool.sse"),
		)
		source := &tokens{}
		drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: source})

		_, err := drv.Chat(t.Context(), request("gpt-6.1-sol"))

		require.NoError(t, err)
		assert.Equal(t, 1, source.renewed)
		assert.Equal(t, "Bearer access-1", fake.Sent()[1].Header.Get("Authorization"))
	})

	t.Run("maps a used-up plan to quota", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(responsesURL, jsonResponse(fhttp.StatusTooManyRequests, apiError("usage_limit_reached", "limit")))
		drv := chatgpt.New(chatgpt.Options{HTTP: hc.StdClient(), Source: &tokens{}})

		_, err := drv.Stream(t.Context(), request("gpt-6.1-sol"))

		assert.Equal(t, contracts.LLMErrorQuota, llmError(t, err).Kind)
		assert.False(t, llmError(t, err).Retryable())
	})
}

/*
jwt builds an unsigned token carrying the claims.
*/
func jwt(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestChatGPTSignIn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	t.Run("runs a device sign-in through to tokens", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		id := jwt(t, map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acct_9"}})
		fake.Stub("POST "+issuer+"/api/accounts/deviceauth/usercode",
			jsonResponse(fhttp.StatusOK, `{"device_auth_id":"dev_1","user_code":"ABCD-1234","interval":"7"}`))
		fake.StubSequence("POST "+issuer+"/api/accounts/deviceauth/token",
			jsonResponse(fhttp.StatusForbidden, `{}`),
			jsonResponse(fhttp.StatusOK, `{"authorization_code":"code_1","code_verifier":"v_1"}`),
		)
		fake.Stub("POST "+exchangeURL, jsonResponse(fhttp.StatusOK,
			`{"access_token":"at","refresh_token":"rt","id_token":"`+id+`","expires_in":3600}`))
		client := &chatgpt.Client{HTTP: hc, ClientID: "app_x", Issuer: issuer, TokenURL: exchangeURL}

		code, err := client.StartDevice(t.Context())
		require.NoError(t, err)
		_, pending := client.PollDevice(t.Context(), code.DeviceAuthID, code.UserCode)
		grant, err := client.PollDevice(t.Context(), code.DeviceAuthID, code.UserCode)
		require.NoError(t, err)
		set, err := client.Exchange(t.Context(), chatgpt.CodeGrant{
			Code: grant.AuthorizationCode, Verifier: grant.CodeVerifier, RedirectURI: client.DeviceRedirectURI(),
		}, now)
		require.NoError(t, err)

		assert.Equal(t, "ABCD-1234", code.UserCode)
		assert.Equal(t, 7*time.Second, code.Interval)
		require.ErrorIs(t, pending, chatgpt.ErrPending)
		assert.Equal(t, "acct_9", set.AccountID)
		assert.Equal(t, now.Add(time.Hour), set.ExpiresAt)
		assert.Equal(t, issuer+"/codex/device", client.VerificationURL())

		exchange := string(fake.Sent()[3].Body)
		assert.Contains(t, exchange, "code_verifier=v_1")
		assert.Contains(t, exchange, "redirect_uri=https%3A%2F%2Fauth.openai.com%2Fdeviceauth%2Fcallback")
	})

	t.Run("reports device sign-in turned off", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub("POST "+issuer+"/api/accounts/deviceauth/usercode", jsonResponse(fhttp.StatusNotFound, `{}`))
		client := &chatgpt.Client{HTTP: hc, ClientID: "app_x", Issuer: issuer}

		_, err := client.StartDevice(t.Context())

		require.ErrorIs(t, err, chatgpt.ErrDeviceUnavailable)
	})

	t.Run("rotates the refresh token and reads expiry from the token", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		access := jwt(t, map[string]any{"exp": float64(now.Add(2 * time.Hour).Unix())})
		fake.StubSequence("POST "+exchangeURL,
			jsonResponse(fhttp.StatusOK, `{"access_token":"`+access+`","refresh_token":"rt_2"}`),
			jsonResponse(fhttp.StatusBadRequest, `{"error":"invalid_grant"}`),
		)
		client := &chatgpt.Client{HTTP: hc, ClientID: "app_x", TokenURL: exchangeURL}

		held := chatgpt.TokenSet{RefreshToken: "rt_1", AccountID: "acct_1", IDToken: "id"}
		set, err := client.Refresh(t.Context(), held, now)
		require.NoError(t, err)
		_, spent := client.Refresh(t.Context(), held, now)

		assert.Equal(t, "rt_2", set.RefreshToken)
		assert.Equal(t, "acct_1", set.AccountID)
		assert.Equal(t, "id", set.IDToken)
		assert.Equal(t, now.Add(2*time.Hour), set.ExpiresAt)
		require.ErrorIs(t, spent, chatgpt.ErrSignInExpired)
		assert.Equal(t, "refresh_token", body(t, fake.Sent()[0])["grant_type"])
		assert.Equal(t, "app_x", body(t, fake.Sent()[0])["client_id"])
	})
}
