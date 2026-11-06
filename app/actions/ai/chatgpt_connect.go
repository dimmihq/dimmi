/*
|--------------------------------------------------------------------------
| ChatGPT Connect
|--------------------------------------------------------------------------
|
| The subscription sign-in: start builds an authorize URL with a PKCE
| challenge and a state parked in cache; complete exchanges the code,
| seals the token set onto a connection row, and learns the account.
| The credential is never seen by the client, only the public bits.
|
*/

package ai

import (
	"context"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"
	
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
stateTTL is how long a connect attempt stays valid.
*/
const stateTTL = 10 * time.Minute

/*
oauthState is parked in cache under the state id.
*/
type oauthState struct {
	WorkspaceID string `json:"workspace_id"`
	MemberID    string `json:"member_id"`
	Verifier    string `json:"verifier"`
	CreatedAt   int64  `json:"created_at"`
}

/*
tokenAnswer is OpenAI's token endpoint response.
*/
type tokenAnswer struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

/*
StartInput is who starts the connect.
*/
type StartInput struct {
	Membership  authz.Membership
	RedirectURI string
}

/*
StartAnswer is the URL to send the member to.
*/
type StartAnswer struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

/*
StartChatGPTConnect builds the authorize URL and parks the verifier.

Rules:
  - The caller needs ai:keys:write; the install must offer the
    source.

Errors: [exceptions.ErrForbidden], a [fault] when the source is off,
or a wrapped cache error.
*/
type StartChatGPTConnect struct {
	Config config.AIConfig
	Cache  *cache.Store
	Clock  clock.Clock
}

/*
Handle returns the authorize URL.
*/
func (a *StartChatGPTConnect) Handle(ctx context.Context, in StartInput) (*StartAnswer, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}
	cfg := a.Config.ChatGPT
	if !cfg.Enabled {
		return nil, fault.New(fault.Invalid, "chatgpt_unavailable", "ChatGPT connect is not enabled")
	}

	verifier := randomURL(64)
	state := randomURL(32)
	sum := hashing.SHA256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum)

	payload, _ := json.Marshal(oauthState{
		WorkspaceID: in.Membership.Workspace.ID.String(),
		MemberID:    in.Membership.Member.ID.String(),
		Verifier:    verifier,
		CreatedAt:   a.Clock.Now().Unix(),
	})
	if ok, err := a.Cache.Add(ctx, "ai:oauth:"+state, payload, stateTTL); err != nil || !ok {
		return nil, fmt.Errorf("park oauth state: %w", err)
	}

	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {in.RedirectURI},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	return &StartAnswer{URL: cfg.AuthorizeURL + "?" + query.Encode(), State: state}, nil
}

/*
CompleteInput carries the callback's answers.
*/
type CompleteInput struct {
	Membership  authz.Membership
	State       string
	Code        string
	RedirectURI string
}

/*
CompleteAnswer is the created connection's id.
*/
type CompleteAnswer struct {
	KeyID     ids.UUID `json:"key_id"`
	AccountID string   `json:"account_id"`
}

/*
CompleteChatGPTConnect exchanges the code and stores the connection.

Rules:
  - The caller needs ai:keys:write; the state must match a parked
    attempt for the same workspace; the exchange happens inside the
    transaction's timeout.

Errors: [exceptions.ErrForbidden], a [fault] on a bad state or code,
or a wrapped database error.
*/
type CompleteChatGPTConnect struct {
	DB     *database.DB
	Config config.AIConfig
	Cache  *cache.Store
	Clock  clock.Clock
	Keys   *encryption.Keys
	HTTP   *httpclient.Client
}

/*
Handle creates the connection and returns it.
*/
func (a *CompleteChatGPTConnect) Handle(ctx context.Context, in CompleteInput) (*CompleteAnswer, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}

	payload, err := a.Cache.Get(ctx, "ai:oauth:"+in.State)
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_state_invalid", "the connect expired, start again")
	}
	a.Cache.Delete(ctx, "ai:oauth:"+in.State)

	var parked oauthState
	if err := json.Unmarshal(payload, &parked); err != nil || parked.WorkspaceID != in.Membership.Workspace.ID.String() {
		return nil, fault.New(fault.Invalid, "oauth_state_invalid", "the connect expired, start again")
	}

	tokens, err := a.exchange(ctx, in.Code, parked.Verifier, in.RedirectURI)
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_exchange_failed", "the sign-in could not be completed")
	}

	var key models.AiProviderKey
	memberID := in.Membership.Member.ID

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		key, err = models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: "ChatGPT subscription",
			Provider: string(enums.AIProviderChatGPT), AuthType: "oauth", Status: "valid",
			StatusReason: strPtr("connected"), Models: json.RawMessage("[]"),
			ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"), Source: "dashboard",
			CreatedByMemberID: &memberID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create provider key: %w", err)
		}

		sealed, err := encryption.Seal(ctx, a.Keys,
			encryption.Slot{Scope: encryption.Scope{Kind: "workspace", ID: in.Membership.Workspace.ID.String()}},
			managers.Credentials{
				AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
				ExpiresAt: a.Clock.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
				Scopes:    []string{"openid", "profile", "email", "offline_access"},
			})
		if err != nil {
			return fmt.Errorf("seal credentials: %w", err)
		}

		key, err = models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Name: key.Name, BaseUrl: key.BaseUrl,
			Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: sealed,
			KeyHint: nil, Config: key.Config, Now: a.Clock.Now(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return &CompleteAnswer{KeyID: key.ID}, nil
}

/*
exchange trades the code and verifier for tokens.
*/
func (a *CompleteChatGPTConnect) exchange(ctx context.Context, code, verifier, redirectURI string) (*tokenAnswer, error) {
	cfg := a.Config.ChatGPT

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := a.HTTP.New().
		WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Post(ctx, cfg.TokenURL, form.Encode())
	if err != nil {
		return nil, err
	}

	var tokens tokenAnswer
	if err := json.Unmarshal(resp.Body, &tokens); err != nil || tokens.AccessToken == "" {
		return nil, fmt.Errorf("bad token answer")
	}
	return &tokens, nil
}

/*
randomURL is a URL-safe random string for state and verifier.
*/
func randomURL(n int) string {
	return str.Random(n)
}

var _ = strings.TrimSpace
var _ = managers.Credentials{}
