/*
|--------------------------------------------------------------------------
| ChatGPT Connect
|--------------------------------------------------------------------------
|
| The subscription sign-in. The OAuth client a ChatGPT plan accepts is
| registered with a loopback redirect only, so a web app cannot catch
| the callback itself. Start offers two ways around that: a device
| code the member types on OpenAI's page while the dashboard polls
| complete, or a browser sign-in whose final, unreachable address the
| member pastes back into complete. Either way the attempt is parked in
| cache under a state id for one workspace, and the credential is never
| seen by the client.
|
*/

package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/config"
)

/*
The two sign-in methods.
*/
const (
	ConnectMethodDevice  = "device"
	ConnectMethodBrowser = "browser"
)

/*
stateTTL is how long a connect attempt stays valid; the issuer's
device codes last as long.
*/
const stateTTL = 15 * time.Minute

/*
oauthState is parked in cache under the state id: the verifier for a
browser sign-in, the device code for a device sign-in, and when the
dashboard last polled.
*/
type oauthState struct {
	WorkspaceID  string `json:"workspace_id"`
	MemberID     string `json:"member_id"`
	Method       string `json:"method"`
	Verifier     string `json:"verifier,omitempty"`
	DeviceAuthID string `json:"device_auth_id,omitempty"`
	UserCode     string `json:"user_code,omitempty"`
	Interval     int    `json:"interval"`
	LastPollAt   int64  `json:"last_poll_at,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

/*
StartInput is who starts the connect and how. Method is "device", the
default, or "browser". RedirectURI is ignored: the subscription only
accepts its registered loopback redirect.
*/
type StartInput struct {
	Membership  authz.Membership
	Method      string
	RedirectURI string
}

/*
StartAnswer tells the dashboard what to show. For a device sign-in,
URL is the page to open and UserCode the code to type there; the
dashboard then polls complete every IntervalSeconds. For a browser
sign-in, URL is the authorize page; after signing in the browser
lands on RedirectURI, which does not load, and the member pastes that
address into complete.
*/
type StartAnswer struct {
	Method          string    `json:"method"`
	State           string    `json:"state"`
	URL             string    `json:"url"`
	UserCode        string    `json:"user_code,omitempty"`
	IntervalSeconds int       `json:"interval_seconds,omitempty"`
	RedirectURI     string    `json:"redirect_uri,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
}

/*
StartChatGPTConnect starts a subscription sign-in.

Rules:
  - The caller needs ai:keys; the install must offer the source.
  - A device sign-in needs device login turned on in the member's
    ChatGPT security settings; without it the browser method remains.

Errors: [exceptions.ErrForbidden], a [fault] when the source is off,
device login is disabled or the method is unknown, or a wrapped cache
or network error.
*/
type StartChatGPTConnect struct {
	Config config.AIConfig
	Cache  *cache.Store
	Clock  clock.Clock
	HTTP   *httpclient.Client
}

/*
Handle starts the sign-in and parks the attempt.
*/
func (a *StartChatGPTConnect) Handle(ctx context.Context, in StartInput) (*StartAnswer, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}
	if !a.Config.ChatGPT.Enabled {
		return nil, fault.New(fault.Invalid, "chatgpt_unavailable", "ChatGPT connect is not enabled")
	}

	now := a.Clock.Now()
	state := str.Random(32)
	parked := oauthState{
		WorkspaceID: in.Membership.Workspace.ID.String(), MemberID: in.Membership.Member.ID.String(),
		CreatedAt: now.Unix(),
	}

	var answer *StartAnswer
	var err error
	switch in.Method {
	case "", ConnectMethodDevice:
		answer, err = a.device(ctx, &parked)
	case ConnectMethodBrowser:
		answer = a.browser(&parked, state)
	default:
		return nil, fault.New(fault.Invalid, "connect_method_invalid", "the method must be device or browser")
	}
	if err != nil {
		return nil, err
	}

	answer.State = state
	answer.ExpiresAt = now.Add(stateTTL)
	payload, _ := json.Marshal(parked)
	if ok, err := a.Cache.Add(ctx, stateKey(answer.State), payload, stateTTL); err != nil || !ok {
		return nil, fmt.Errorf("park oauth state: %w", err)
	}
	return answer, nil
}

/*
device asks the issuer for a device code.
*/
func (a *StartChatGPTConnect) device(ctx context.Context, parked *oauthState) (*StartAnswer, error) {
	client := signInClient(a.Config, a.HTTP)
	code, err := client.StartDevice(ctx)
	if errors.Is(err, chatgpt.ErrDeviceUnavailable) {
		return nil, fault.New(fault.Invalid, "chatgpt_device_disabled",
			"device sign-in is off for this ChatGPT account: turn it on in ChatGPT's security "+
				"settings, or sign in with the browser")
	}
	if err != nil {
		return nil, fmt.Errorf("start chatgpt device sign-in: %w", err)
	}

	parked.Method = ConnectMethodDevice
	parked.DeviceAuthID, parked.UserCode = code.DeviceAuthID, code.UserCode
	parked.Interval = int(code.Interval / time.Second)
	return &StartAnswer{
		Method: ConnectMethodDevice, URL: client.VerificationURL(),
		UserCode: code.UserCode, IntervalSeconds: parked.Interval,
	}, nil
}

/*
browser builds the authorize URL with a PKCE challenge; the attempt's
state rides along, so the address the member pastes back names it.
*/
func (a *StartChatGPTConnect) browser(parked *oauthState, state string) *StartAnswer {
	cfg := a.Config.ChatGPT
	verifier := str.Random(64)
	challenge := base64.RawURLEncoding.EncodeToString(hashing.SHA256([]byte(verifier)))
	parked.Method, parked.Verifier = ConnectMethodBrowser, verifier

	query := url.Values{
		"response_type":              {"code"},
		"client_id":                  {cfg.ClientID},
		"redirect_uri":               {cfg.RedirectURI},
		"scope":                      {"openid profile email offline_access"},
		"state":                      {state},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"originator":                 {cfg.Originator},
	}
	return &StartAnswer{
		Method: ConnectMethodBrowser, URL: cfg.AuthorizeURL + "?" + query.Encode(), RedirectURI: cfg.RedirectURI,
	}
}

/*
signInClient builds the sign-in client from the configuration.
*/
func signInClient(cfg config.AIConfig, http *httpclient.Client) *chatgpt.Client {
	return &chatgpt.Client{
		HTTP: http, ClientID: cfg.ChatGPT.ClientID, Issuer: cfg.ChatGPT.Issuer, TokenURL: cfg.ChatGPT.TokenURL,
	}
}

/*
stateKey is where an attempt is parked.
*/
func stateKey(state string) string { return "ai:oauth:" + state }
