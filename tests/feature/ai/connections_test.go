package ai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/llm"
)

const (
	exchangeURL  = "POST https://auth.openai.com/oauth/token"
	responsesURL = "POST https://chatgpt.com/backend-api/codex/responses"
)

var errRollback = errors.New("roll the run back")

/*
keysOwner is the workspace owner allowed to manage connections, and a
context scoped to the workspace.
*/
func keysOwner(tc *tests.TestCase) (authz.Membership, context.Context) {
	m := authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner, []string{"ai:keys"})
	return m, tenancy.With(tc.T.Context(), tc.Workspace.Workspace.ID.String())
}

/*
jsonReply is a JSON response.
*/
func jsonReply(status int, body string) httpclient.Response {
	return httpclient.Response{
		Status: status, Header: fhttp.Header{"Content-Type": {"application/json"}}, Body: []byte(body),
	}
}

/*
idToken is an unsigned ID token naming the plan account.
*/
func idToken(account string) string {
	claims, _ := json.Marshal(map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": account},
	})
	return "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
}

/*
loadKey reads a connection and opens its credentials.
*/
func loadKey(t *testing.T, tc *tests.TestCase, id ids.UUID) (models.AiProviderKey, managers.Credentials) {
	t.Helper()
	var key models.AiProviderKey
	var creds managers.Credentials
	tc.InWorkspace(func(ctx context.Context) error {
		var err error
		key, err = models.New(tc.App.DB).FindProviderKey(ctx, models.FindProviderKeyParams{
			WorkspaceID: tc.Workspace.Workspace.ID, ID: id,
		})
		if err != nil {
			return err
		}
		creds, err = managers.OpenCredentials(ctx, tc.App.Keys, key)
		return err
	})
	return key, creds
}

/*
planKey stores a subscription connection whose bearer has expired.
*/
func planKey(tc *tests.TestCase) models.AiProviderKey {
	var key models.AiProviderKey
	tc.InWorkspace(func(ctx context.Context) error {
		var err error
		key, err = models.New(tc.App.DB).CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: tc.Workspace.Workspace.ID, Name: "ChatGPT", Provider: "chatgpt", AuthType: "oauth",
			Status: "valid", Models: json.RawMessage("[]"), ProviderInfo: json.RawMessage("{}"),
			Config: json.RawMessage("{}"), Source: "dashboard", Now: tc.Clock.Now(),
		})
		if err != nil {
			return err
		}
		sealed, err := managers.SealCredentials(ctx, tc.App.Keys, key, managers.Credentials{
			AccessToken: "old", RefreshToken: "rt_1", AccountID: "acct_1", ExpiresAt: tc.Clock.Now().Add(-time.Minute),
		})
		if err != nil {
			return err
		}
		key, err = models.New(tc.App.DB).UpdateProviderKeyCredentials(ctx, models.UpdateProviderKeyCredentialsParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, CredentialsEncrypted: sealed, Now: tc.Clock.Now(),
		})
		return err
	})
	return key
}

func hello() contracts.ChatRequest {
	return contracts.ChatRequest{
		Model:    "gpt-6.1-sol",
		Messages: []contracts.ChatMessage{{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "hi"}}}},
	}
}

func TestProviderConnections(t *testing.T) {
	t.Parallel()

	t.Run("seals an api key that the manager opens and sends", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		fake.Stub("GET https://api.anthropic.com/v1/models*", fixtures.Reply("anthropic_models.json"))
		fake.Stub("POST https://api.anthropic.com/v1/messages", fixtures.Reply("anthropic_message.json"))
		m, ctx := keysOwner(tc)

		pasted := "sk-ant-live-1234"
		key, err := tc.App.Actions.AI.CreateProviderKey.Handle(ctx, ai.CreateKeyInput{
			Membership: m, Name: "Anthropic", Provider: enums.AIProviderAnthropic, APIKey: pasted,
		})
		require.NoError(t, err)
		assert.Equal(t, "valid", key.Status)

		drv, err := tc.App.LLM.ForKey(ctx, *key)
		require.NoError(t, err)
		req := hello()
		req.Model = "claude-opus-5-5"
		res, err := drv.Chat(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "Hello", res.Text)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.URL.Path == "/v1/messages" && req.Header.Get("X-Api-Key") == "sk-ant-live-1234"
		})
	})

	t.Run("refuses to send a credential over plain http", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		m, ctx := keysOwner(tc)

		_, err := tc.App.Actions.AI.CreateProviderKey.Handle(ctx, ai.CreateKeyInput{
			Membership: m, Name: "Gateway", Provider: enums.AIProviderOpenAICompatible,
			BaseURL: "http://llm.example.com/v1", APIKey: "sk-1",
		})

		var f *fault.Error
		require.ErrorAs(t, err, &f)
		assert.Equal(t, "base_url_insecure", f.Code())
	})

	t.Run("keeps a renewed subscription token when the run rolls back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		fake.Stub(exchangeURL, jsonReply(fhttp.StatusOK,
			`{"access_token":"new","refresh_token":"rt_2","id_token":"`+idToken("acct_2")+`","expires_in":3600}`))
		fake.Stub(responsesURL, fixtures.Reply("chatgpt_reply.sse"))
		key := planKey(tc)
		_, ctx := keysOwner(tc)

		err := tc.App.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
			drv, err := tc.App.LLM.ForKey(ctx, key)
			require.NoError(t, err)
			res, err := drv.Chat(ctx, hello())
			require.NoError(t, err)
			assert.Equal(t, "Hi", res.Text)
			return errRollback
		})
		require.ErrorIs(t, err, errRollback)

		_, creds := loadKey(t, tc, key.ID)
		assert.Equal(t, "rt_2", creds.RefreshToken)
		assert.Equal(t, "new", creds.AccessToken)
		assert.Equal(t, "acct_2", creds.AccountID)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.URL.Host == "chatgpt.com" && req.Header.Get("Authorization") == "Bearer new" &&
				req.Header.Get("chatgpt-account-id") == "acct_2"
		})
	})

	t.Run("takes a token another worker already renewed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		fake.Stub(exchangeURL, jsonReply(fhttp.StatusOK,
			`{"access_token":"new","refresh_token":"rt_2","expires_in":3600}`))
		fake.Stub(responsesURL, fixtures.Reply("chatgpt_reply.sse"))
		key := planKey(tc)
		_, ctx := keysOwner(tc)

		for range 2 {
			drv, err := tc.App.LLM.ForKey(ctx, key)
			require.NoError(t, err)
			_, err = drv.Chat(ctx, hello())
			require.NoError(t, err)
			tc.App.LLM = managers.NewLLMManager(managers.LLMDeps{
				Keys: tc.App.Keys, HTTP: tc.App.HTTP, Config: tc.App.Config.AI, Clock: tc.Clock, DB: tc.App.DB,
			})
		}

		refreshes := 0
		for _, req := range fake.Sent() {
			if req.URL.Host == "auth.openai.com" {
				refreshes++
			}
		}
		assert.Equal(t, 1, refreshes)
	})
}

func TestChatGPTConnect(t *testing.T) {
	t.Parallel()

	enabled := tests.WithEnv("DIMMI_AI__CHATGPT__ENABLED=true")

	t.Run("connects a plan through a device code", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, enabled)
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		fake.Stub("POST https://auth.openai.com/api/accounts/deviceauth/usercode",
			jsonReply(fhttp.StatusOK, `{"device_auth_id":"dev_1","user_code":"ABCD-1234","interval":"5"}`))
		fake.StubSequence("POST https://auth.openai.com/api/accounts/deviceauth/token",
			jsonReply(fhttp.StatusForbidden, `{}`),
			jsonReply(fhttp.StatusOK, `{"authorization_code":"code_1","code_challenge":"c","code_verifier":"v_1"}`))
		fake.Stub(exchangeURL, jsonReply(fhttp.StatusOK,
			`{"access_token":"at","refresh_token":"rt","id_token":"`+idToken("acct_9")+`","expires_in":3600}`))
		m, ctx := keysOwner(tc)

		start, err := tc.App.Actions.AI.StartChatGPTConnect.Handle(ctx, ai.StartInput{Membership: m})
		require.NoError(t, err)
		poll := ai.CompleteInput{Membership: m, State: start.State}
		first, err := tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, poll)
		require.NoError(t, err)
		early, err := tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, poll)
		require.NoError(t, err)
		tc.Clock.Travel(5 * time.Second)
		done, err := tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, poll)
		require.NoError(t, err)

		assert.Equal(t, "https://auth.openai.com/codex/device", start.URL)
		assert.Equal(t, "ABCD-1234", start.UserCode)
		assert.Equal(t, 5, start.IntervalSeconds)
		assert.Equal(t, ai.ConnectStatusPending, first.Status)
		assert.Equal(t, ai.ConnectStatusPending, early.Status)
		assert.Equal(t, ai.ConnectStatusConnected, done.Status)
		assert.Equal(t, "acct_9", done.AccountID)

		key, creds := loadKey(t, tc, done.KeyID)
		assert.Equal(t, "chatgpt", key.Provider)
		assert.Equal(t, "rt", creds.RefreshToken)
		assert.Equal(t, "acct_9", creds.AccountID)

		_, err = tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, poll)
		var f *fault.Error
		require.ErrorAs(t, err, &f)
		assert.Equal(t, "oauth_state_invalid", f.Code())
	})

	t.Run("connects a plan from the pasted sign-in address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, enabled)
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		fake.Stub(exchangeURL, jsonReply(fhttp.StatusOK,
			`{"access_token":"at","refresh_token":"rt","id_token":"`+idToken("acct_3")+`","expires_in":3600}`))
		m, ctx := keysOwner(tc)

		start, err := tc.App.Actions.AI.StartChatGPTConnect.Handle(ctx, ai.StartInput{Membership: m, Method: "browser"})
		require.NoError(t, err)
		authorize, err := url.Parse(start.URL)
		require.NoError(t, err)
		_, mismatch := tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, ai.CompleteInput{
			Membership: m, State: start.State, Code: start.RedirectURI + "?code=c1&state=other",
		})
		done, err := tc.App.Actions.AI.CompleteChatGPTConnect.Handle(ctx, ai.CompleteInput{
			Membership: m, State: start.State, Code: start.RedirectURI + "?code=c1&state=" + start.State,
		})
		require.NoError(t, err)

		assert.Equal(t, start.State, authorize.Query().Get("state"))
		assert.Equal(t, "S256", authorize.Query().Get("code_challenge_method"))
		assert.Equal(t, "http://localhost:1455/auth/callback", authorize.Query().Get("redirect_uri"))
		var f *fault.Error
		require.ErrorAs(t, mismatch, &f)
		assert.Equal(t, "oauth_state_mismatch", f.Code())
		assert.Equal(t, ai.ConnectStatusConnected, done.Status)
		assert.Equal(t, "acct_3", done.AccountID)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			form, _ := url.ParseQuery(string(req.Body))
			return form.Get("code") == "c1" && form.Get("code_verifier") != ""
		})
	})

	t.Run("refuses to start while the source is off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		m, ctx := keysOwner(tc)

		_, err := tc.App.Actions.AI.StartChatGPTConnect.Handle(ctx, ai.StartInput{Membership: m})

		var f *fault.Error
		require.ErrorAs(t, err, &f)
		assert.Equal(t, "chatgpt_unavailable", f.Code())
	})
}
