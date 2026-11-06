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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/config"
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
randomURL is a URL-safe random string for state and verifier.
*/
func randomURL(n int) string {
	return str.Random(n)
}

var (
	_ = strings.TrimSpace
	_ = managers.Credentials{}
)
