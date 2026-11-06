/*
|--------------------------------------------------------------------------
| ChatGPT Connect Complete
|--------------------------------------------------------------------------
|
| The callback: the code exchanges for tokens, the token set seals
| onto a new connection row, and the account learns what it can.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

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
	_ = a.Cache.Delete(ctx, "ai:oauth:"+in.State)

	var parked oauthState
	if err := json.Unmarshal(payload, &parked); err != nil ||
		parked.WorkspaceID != in.Membership.Workspace.ID.String() {
		return nil, fault.New(fault.Invalid, "oauth_state_invalid", "the connect expired, start again")
	}

	tokens, err := a.exchange(ctx, in.Code, parked.Verifier, in.RedirectURI)
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_exchange_failed", "the sign-in could not be completed")
	}

	var key models.AiProviderKey

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err := a.store(ctx, tx, in.Membership, tokens)
		key = out
		return err
	})
	if err != nil {
		return nil, err
	}

	return &CompleteAnswer{KeyID: key.ID}, nil
}

/*
store writes the connection with its sealed token set.
*/
func (a *CompleteChatGPTConnect) store(ctx context.Context, tx *database.Tx,
	m authz.Membership, tokens *tokenAnswer,
) (models.AiProviderKey, error) {
	memberID := m.Member.ID
	key, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: m.Workspace.ID, Name: "ChatGPT subscription",
		Provider: string(enums.AIProviderChatGPT), AuthType: "oauth", Status: "valid",
		StatusReason: new("connected"), Models: json.RawMessage("[]"),
		ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"), Source: "dashboard",
		CreatedByMemberID: &memberID, Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("create provider key: %w", err)
	}

	sealed, err := encryption.Seal(ctx, a.Keys,
		encryption.Slot{Scope: encryption.Scope{Kind: "workspace", ID: m.Workspace.ID.String()}},
		managers.Credentials{
			AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
			ExpiresAt: a.Clock.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
			Scopes:    []string{"openid", "profile", "email", "offline_access"},
		})
	if err != nil {
		return key, fmt.Errorf("seal credentials: %w", err)
	}

	return models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Name: key.Name, BaseUrl: key.BaseUrl,
		Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: sealed,
		KeyHint: nil, Config: key.Config, Now: a.Clock.Now(),
	})
}

/*
exchange trades the code and verifier for tokens.
*/
func (a *CompleteChatGPTConnect) exchange(ctx context.Context,
	code, verifier, redirectURI string,
) (*tokenAnswer, error) {
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
