/*
|--------------------------------------------------------------------------
| ChatGPT Sign-In
|--------------------------------------------------------------------------
|
| A ChatGPT plan is reached with the OAuth client the ChatGPT coding
| tools share. Its redirect is registered on a loopback address only,
| so a web app signs in one of two ways: a device code the member
| enters on OpenAI's page while the app polls, or a browser sign-in
| whose final address the member pastes back. Both end in the same
| code exchange, and the token set refreshes with a rotating refresh
| token. The account id rides in the ID token's claims.
|
*/

package chatgpt

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Client is the sign-in side of the subscription: the OAuth client id,
the issuer hosting the device-code endpoints and the token URL.
*/
type Client struct {
	HTTP     *httpclient.Client
	ClientID string
	Issuer   string
	TokenURL string
}

/*
TokenSet is what a sign-in or refresh returns.
*/
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	AccountID    string
	ExpiresAt    time.Time
}

/*
DeviceCode is a started device sign-in: the code the member types on
the verification page, and how often the app may poll.
*/
type DeviceCode struct {
	DeviceAuthID string
	UserCode     string
	Interval     time.Duration
}

/*
DeviceGrant is what a finished device sign-in hands back for the code
exchange.
*/
type DeviceGrant struct {
	AuthorizationCode string
	CodeVerifier      string
}

/*
Errors the sign-in reports. ErrPending means the member has not
finished the device sign-in yet; ErrDeviceUnavailable means the
account has device sign-in turned off; ErrSignInExpired means the
refresh token is spent and the member must connect again.
*/
var (
	ErrPending           = errors.New("chatgpt: sign-in pending")
	ErrDeviceUnavailable = errors.New("chatgpt: device sign-in is not enabled for this account")
	ErrSignInExpired     = errors.New("chatgpt: sign-in expired")
	ErrSignInFailed      = errors.New("chatgpt: sign-in failed")
)

/*
defaultInterval is the poll pace when the issuer names none.
*/
const defaultInterval = 5 * time.Second

/*
VerificationURL is the page a member enters the device code on.
*/
func (c *Client) VerificationURL() string { return strings.TrimRight(c.Issuer, "/") + "/codex/device" }

/*
DeviceRedirectURI is the redirect a device grant is exchanged with.
*/
func (c *Client) DeviceRedirectURI() string {
	return strings.TrimRight(c.Issuer, "/") + "/deviceauth/callback"
}

/*
StartDevice asks the issuer for a device code.
*/
func (c *Client) StartDevice(ctx context.Context) (*DeviceCode, error) {
	resp, err := c.HTTP.New().Post(ctx, c.issuerURL("/api/accounts/deviceauth/usercode"),
		map[string]string{"client_id": c.ClientID})
	if err != nil {
		return nil, fmt.Errorf("start device sign-in: %w", err)
	}
	if resp.Status == fhttp.StatusNotFound {
		return nil, ErrDeviceUnavailable
	}
	if resp.Status/100 != 2 {
		return nil, fmt.Errorf("start device sign-in: status %d: %w", resp.Status, ErrSignInFailed)
	}

	var body struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
		UserCodeAlt  string `json:"usercode"`
		Interval     any    `json:"interval"`
	}
	if err := resp.JSON(&body); err != nil || body.DeviceAuthID == "" {
		return nil, fmt.Errorf("start device sign-in: unreadable answer: %w", ErrSignInFailed)
	}
	code := body.UserCode
	if code == "" {
		code = body.UserCodeAlt
	}
	return &DeviceCode{DeviceAuthID: body.DeviceAuthID, UserCode: code, Interval: interval(body.Interval)}, nil
}

/*
PollDevice asks whether the member finished the device sign-in;
ErrPending means not yet.
*/
func (c *Client) PollDevice(ctx context.Context, deviceAuthID, userCode string) (*DeviceGrant, error) {
	resp, err := c.HTTP.New().Post(ctx, c.issuerURL("/api/accounts/deviceauth/token"),
		map[string]string{"device_auth_id": deviceAuthID, "user_code": userCode})
	if err != nil {
		return nil, fmt.Errorf("poll device sign-in: %w", err)
	}
	if resp.Status == fhttp.StatusForbidden || resp.Status == fhttp.StatusNotFound {
		return nil, ErrPending
	}
	if resp.Status/100 != 2 {
		return nil, fmt.Errorf("poll device sign-in: status %d: %w", resp.Status, ErrSignInFailed)
	}

	var body struct {
		AuthorizationCode string `json:"authorization_code"`
		CodeVerifier      string `json:"code_verifier"`
	}
	if err := resp.JSON(&body); err != nil || body.AuthorizationCode == "" {
		return nil, fmt.Errorf("poll device sign-in: unreadable answer: %w", ErrSignInFailed)
	}
	return &DeviceGrant{AuthorizationCode: body.AuthorizationCode, CodeVerifier: body.CodeVerifier}, nil
}

/*
CodeGrant is an authorization code with the PKCE verifier and the
redirect it was issued for.
*/
type CodeGrant struct {
	Code        string
	Verifier    string
	RedirectURI string
}

/*
Exchange trades an authorization code and its PKCE verifier for a
token set.
*/
func (c *Client) Exchange(ctx context.Context, grant CodeGrant, now time.Time) (*TokenSet, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"code":          {grant.Code},
		"redirect_uri":  {grant.RedirectURI},
		"code_verifier": {grant.Verifier},
	}
	resp, err := c.HTTP.New().
		WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Post(ctx, c.TokenURL, form.Encode())
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	if resp.Status/100 != 2 {
		return nil, fmt.Errorf("exchange code: status %d: %w", resp.Status, ErrSignInFailed)
	}
	return readTokens(resp, nil, now)
}

/*
Refresh renews a token set. Refresh tokens rotate, so the answer's
token replaces the old one, which is kept only when none came back.
*/
func (c *Client) Refresh(ctx context.Context, current TokenSet, now time.Time) (*TokenSet, error) {
	resp, err := c.HTTP.New().Post(ctx, c.TokenURL, map[string]string{
		"grant_type": "refresh_token", "client_id": c.ClientID, "refresh_token": current.RefreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if resp.Status == fhttp.StatusBadRequest || resp.Status == fhttp.StatusUnauthorized {
		return nil, ErrSignInExpired
	}
	if resp.Status/100 != 2 {
		return nil, fmt.Errorf("refresh token: status %d: %w", resp.Status, ErrSignInFailed)
	}
	return readTokens(resp, &current, now)
}

/*
readTokens decodes a token answer, keeping the current values the
answer left out.
*/
func readTokens(resp *httpclient.Response, current *TokenSet, now time.Time) (*TokenSet, error) {
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := resp.JSON(&body); err != nil || body.AccessToken == "" {
		return nil, fmt.Errorf("read tokens: unreadable answer: %w", ErrSignInFailed)
	}

	out := TokenSet{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken, IDToken: body.IDToken}
	if current != nil {
		out.RefreshToken = cmp.Or(out.RefreshToken, current.RefreshToken)
		out.IDToken = cmp.Or(out.IDToken, current.IDToken)
		out.AccountID = current.AccountID
	}
	if id := AccountID(out.IDToken, out.AccessToken); id != "" {
		out.AccountID = id
	}
	out.ExpiresAt = expiry(out.AccessToken, body.ExpiresIn, now)
	return &out, nil
}

/*
AccountID reads the ChatGPT account id from the ID token's claims,
then the access token's, looking where the issuer puts it.
*/
func AccountID(tokens ...string) string {
	for _, token := range tokens {
		claims := jwtClaims(token)
		if id, _ := claims["chatgpt_account_id"].(string); id != "" {
			return id
		}
		if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			if id, _ := auth["chatgpt_account_id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

/*
expiry is when an access token stops working: from expires_in when
the answer has it, else from the token's own exp claim, else an hour.
*/
func expiry(accessToken string, expiresIn int64, now time.Time) time.Time {
	if expiresIn > 0 {
		return now.Add(time.Duration(expiresIn) * time.Second)
	}
	if exp, ok := jwtClaims(accessToken)["exp"].(float64); ok && exp > 0 {
		return time.Unix(int64(exp), 0).UTC()
	}
	return now.Add(time.Hour)
}

/*
jwtClaims decodes a JWT's payload without checking its signature: the
token came straight from the issuer over TLS, and only routing facts
are read from it.
*/
func jwtClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

/*
interval reads the poll pace, sent as a number or a string of
seconds.
*/
func interval(v any) time.Duration {
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return time.Duration(n) * time.Second
		}
	case string:
		if secs, err := strconv.Atoi(strings.TrimSpace(n)); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return defaultInterval
}

/*
issuerURL joins a path to the issuer.
*/
func (c *Client) issuerURL(path string) string {
	return strings.TrimRight(c.Issuer, "/") + path
}
