/*
|--------------------------------------------------------------------------
| ChatGPT Connect Complete
|--------------------------------------------------------------------------
|
| The second half of the sign-in. A device attempt is polled: until the
| member finishes on OpenAI's page the answer is pending, and polls
| faster than the issuer allows are answered without asking it. A
| browser attempt takes the address the member pasted, checks its
| state, and exchanges its code. Both end with the token set sealed
| onto a new connection row bound to that row's id.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/config"
)

/*
The connect statuses complete answers with.
*/
const (
	ConnectStatusPending   = "pending"
	ConnectStatusConnected = "connected"
)

/*
CompleteInput carries the attempt's state and, for a browser sign-in,
the address the browser landed on or the bare code from it.
RedirectURI is ignored.
*/
type CompleteInput struct {
	Membership  authz.Membership
	State       string
	Code        string
	RedirectURI string
}

/*
CompleteAnswer is pending, with when to poll again, or connected,
with the new connection's id and the plan's account id.
*/
type CompleteAnswer struct {
	Status          string   `json:"status"`
	IntervalSeconds int      `json:"interval_seconds,omitempty"`
	KeyID           ids.UUID `json:"key_id,omitzero"`
	AccountID       string   `json:"account_id,omitempty"`
}

/*
CompleteChatGPTConnect finishes a subscription sign-in.

Rules:
  - The caller needs ai:keys; the state must match a parked attempt
    of the same workspace; a browser attempt needs the pasted address
    or code, whose own state must match.

Errors: [exceptions.ErrForbidden], a [fault] on an expired or foreign
state, a missing or mismatched code or a failed exchange, or a
wrapped database error.
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
Handle polls or exchanges, and creates the connection once signed in.
*/
func (a *CompleteChatGPTConnect) Handle(ctx context.Context, in CompleteInput) (*CompleteAnswer, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}

	held, release, err := a.Cache.TryLock(ctx, "ai:oauth:lock:"+in.State)
	if errors.Is(err, cache.ErrLocked) {
		return &CompleteAnswer{Status: ConnectStatusPending}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock oauth state: %w", err)
	}
	defer release()

	parked, err := a.parked(held, in)
	if err != nil {
		return nil, err
	}

	var tokens *chatgpt.TokenSet
	if parked.Method == ConnectMethodDevice {
		tokens, err = a.poll(held, in.State, parked)
	} else {
		tokens, err = a.exchangeBrowser(held, in, parked)
	}
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		return &CompleteAnswer{Status: ConnectStatusPending, IntervalSeconds: parked.Interval}, nil
	}

	_ = a.Cache.Delete(held, stateKey(in.State))
	return a.connect(held, in.Membership, tokens)
}

/*
parked loads the attempt and checks it belongs to the workspace.
*/
func (a *CompleteChatGPTConnect) parked(ctx context.Context, in CompleteInput) (oauthState, error) {
	var parked oauthState
	payload, err := a.Cache.Get(ctx, stateKey(in.State))
	if err != nil || json.Unmarshal(payload, &parked) != nil ||
		parked.WorkspaceID != in.Membership.Workspace.ID.String() {
		return parked, fault.New(fault.Invalid, "oauth_state_invalid", "the connect expired, start again")
	}
	return parked, nil
}

/*
poll asks the issuer whether the device sign-in finished, at most
once per interval; nil tokens and no error mean pending.
*/
func (a *CompleteChatGPTConnect) poll(ctx context.Context, state string, parked oauthState) (*chatgpt.TokenSet, error) {
	now := a.Clock.Now()
	if parked.LastPollAt > 0 && now.Unix()-parked.LastPollAt < int64(parked.Interval) {
		return nil, nil
	}
	parked.LastPollAt = now.Unix()
	if ttl := time.Unix(parked.CreatedAt, 0).Add(stateTTL).Sub(now); ttl > 0 {
		payload, _ := json.Marshal(parked)
		_ = a.Cache.Set(ctx, stateKey(state), payload, ttl)
	}

	client := signInClient(a.Config, a.HTTP)
	grant, err := client.PollDevice(ctx, parked.DeviceAuthID, parked.UserCode)
	if errors.Is(err, chatgpt.ErrPending) {
		return nil, nil
	}
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_exchange_failed", "the sign-in could not be completed")
	}

	tokens, err := client.Exchange(ctx, chatgpt.CodeGrant{
		Code: grant.AuthorizationCode, Verifier: grant.CodeVerifier, RedirectURI: client.DeviceRedirectURI(),
	}, now)
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_exchange_failed", "the sign-in could not be completed")
	}
	return tokens, nil
}

/*
exchangeBrowser reads the pasted address and exchanges its code.
*/
func (a *CompleteChatGPTConnect) exchangeBrowser(ctx context.Context, in CompleteInput,
	parked oauthState,
) (*chatgpt.TokenSet, error) {
	code, state := pastedCode(in.Code)
	if code == "" {
		return nil, fault.New(fault.Invalid, "oauth_code_required", "paste the address the sign-in ended on")
	}
	if state != "" && state != in.State {
		return nil, fault.New(fault.Invalid, "oauth_state_mismatch", "that address belongs to another sign-in")
	}

	_ = a.Cache.Delete(ctx, stateKey(in.State))
	tokens, err := signInClient(a.Config, a.HTTP).Exchange(ctx, chatgpt.CodeGrant{
		Code: code, Verifier: parked.Verifier, RedirectURI: a.Config.ChatGPT.RedirectURI,
	}, a.Clock.Now())
	if err != nil {
		return nil, fault.New(fault.Invalid, "oauth_exchange_failed", "the sign-in could not be completed")
	}
	return tokens, nil
}

/*
pastedCode reads the code and state from a pasted callback address,
or takes the input as a bare code.
*/
func pastedCode(input string) (code, state string) {
	input = strings.TrimSpace(input)
	if !strings.Contains(input, "code=") {
		return input, ""
	}
	raw := input
	if _, query, ok := strings.Cut(input, "?"); ok {
		raw = query
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", ""
	}
	return values.Get("code"), values.Get("state")
}

/*
connect stores the connection with its sealed token set.
*/
func (a *CompleteChatGPTConnect) connect(ctx context.Context, m authz.Membership,
	tokens *chatgpt.TokenSet,
) (*CompleteAnswer, error) {
	var key models.AiProviderKey
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		key, err = a.store(ctx, tx, m, tokens)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &CompleteAnswer{Status: ConnectStatusConnected, KeyID: key.ID, AccountID: tokens.AccountID}, nil
}

/*
store writes the connection, then seals the token set bound to its id.
*/
func (a *CompleteChatGPTConnect) store(ctx context.Context, tx *database.Tx,
	m authz.Membership, tokens *chatgpt.TokenSet,
) (models.AiProviderKey, error) {
	var offered []string
	for _, e := range aimodels.ChatGPT() {
		offered = append(offered, e.ID)
	}
	modelsJSON, _ := json.Marshal(offered)

	memberID := m.Member.ID
	key, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: m.Workspace.ID, Name: "ChatGPT subscription",
		Provider: string(enums.AIProviderChatGPT), AuthType: "oauth", Status: "valid",
		StatusReason: new("connected"), Models: modelsJSON,
		ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"), Source: "dashboard",
		CreatedByMemberID: &memberID, LastValidatedAt: new(a.Clock.Now()), Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("create provider key: %w", err)
	}

	sealed, err := managers.SealCredentials(ctx, a.Keys, key, managers.Credentials{
		AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, IDToken: tokens.IDToken,
		AccountID: tokens.AccountID, ExpiresAt: tokens.ExpiresAt,
		Scopes: []string{"openid", "profile", "email", "offline_access"},
	})
	if err != nil {
		return key, err
	}

	key, err = models.New(tx).UpdateProviderKeyCredentials(ctx, models.UpdateProviderKeyCredentialsParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, CredentialsEncrypted: sealed, Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("store provider credentials: %w", err)
	}
	return key, nil
}
