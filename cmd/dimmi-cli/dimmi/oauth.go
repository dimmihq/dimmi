package dimmi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

/*
DefaultURL is dimmi cloud. Self-hosted installs pass their own address
to connect.
*/
const DefaultURL = "https://trydimmi.com"

/*
ClientID is the dimmi CLI's first-party OAuth client: a public native
client that signs in with PKCE through a loopback redirect on any port.
*/
const ClientID = "dimmi-cli"

/*
Scopes are what the CLI asks for: settings:write, which the migrations
API requires, and a refresh token
so a long migration or a sync that runs for weeks stays signed in.
*/
const Scopes = "settings:write offline_access"

/*
Token is a saved dimmi sign-in.
*/
type Token struct {
	BaseURL      string    `json:"base_url"`
	ClientID     string    `json:"client_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

/*
Metadata is the part of dimmi's OAuth server metadata the CLI uses.
*/
type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

/*
Discover reads the OAuth server metadata of the dimmi at base.
*/
func Discover(ctx context.Context, hc *http.Client, base string) (Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(base, "/")+"/.well-known/oauth-authorization-server", nil)
	if err != nil {
		return Metadata{}, err
	}

	res, err := hc.Do(req)
	if err != nil {
		return Metadata{}, err
	}

	var m Metadata
	if err := decode(res, &m); err != nil {
		return m, fmt.Errorf("read dimmi's sign-in settings: %w", err)
	}

	if m.AuthorizationEndpoint == "" || m.TokenEndpoint == "" {
		return m, errors.New("dimmi's sign-in settings name no authorization or token endpoint")
	}

	return m, nil
}

/*
Flow is one sign-in: the PKCE verifier and the state that ties the
answer to the request.
*/
type Flow struct {
	Meta        Metadata
	ClientID    string
	RedirectURI string
	State       string
	Verifier    string
}

/*
NewFlow starts a sign-in that returns to the redirect URI.
*/
func NewFlow(meta Metadata, clientID, redirect string) (*Flow, error) {
	state, err := randomString(24)
	if err != nil {
		return nil, err
	}

	verifier, err := randomString(48)
	if err != nil {
		return nil, err
	}

	return &Flow{Meta: meta, ClientID: clientID, RedirectURI: redirect, State: state, Verifier: verifier}, nil
}

/*
AuthorizeURL is where the person signs in and approves the CLI.
*/
func (f *Flow) AuthorizeURL() string {
	sum := sha256.Sum256([]byte(f.Verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {f.ClientID},
		"redirect_uri":          {f.RedirectURI},
		"scope":                 {Scopes},
		"state":                 {f.State},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}

	sep := "?"
	if strings.Contains(f.Meta.AuthorizationEndpoint, "?") {
		sep = "&"
	}

	return f.Meta.AuthorizationEndpoint + sep + q.Encode()
}

/*
Exchange trades the code the redirect brought for a token. The state
must match the one sent.
*/
func (f *Flow) Exchange(ctx context.Context, hc *http.Client, base string, query url.Values) (Token, error) {
	if e := query.Get("error"); e != "" {
		return Token{}, fmt.Errorf("sign-in was not approved: %s", e)
	}

	if query.Get("state") != f.State {
		return Token{}, errors.New("the sign-in answer doesn't match this request")
	}

	if iss := query.Get("iss"); iss != "" && f.Meta.Issuer != "" && iss != f.Meta.Issuer {
		return Token{}, errors.New("the sign-in answer came from another server")
	}

	code := query.Get("code")
	if code == "" {
		return Token{}, errors.New("the sign-in answer has no code")
	}

	return tokenRequest(ctx, hc, f.Meta.TokenEndpoint, base, f.ClientID, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.RedirectURI},
		"client_id":     {f.ClientID},
		"code_verifier": {f.Verifier},
	})
}

func tokenRequest(ctx context.Context, hc *http.Client, endpoint, base, clientID string, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := hc.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer res.Body.Close()

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}

	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return Token{}, fmt.Errorf("read the token answer: %w", err)
	}

	if res.StatusCode != http.StatusOK || out.AccessToken == "" {
		return Token{}, fmt.Errorf("dimmi refused the sign-in: %s %s", out.Error, out.Description)
	}

	return Token{
		BaseURL: base, ClientID: clientID, AccessToken: out.AccessToken, RefreshToken: out.RefreshToken,
		ExpiresAt: time.Now().Add(time.Duration(out.ExpiresIn) * time.Second), Scope: out.Scope,
	}, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

/*
SavedTokens serves the saved token and refreshes it with its refresh
token, saving the new one through Save.
*/
type SavedTokens struct {
	HTTP *http.Client
	Load func() (Token, error)
	Save func(Token) error

	mu  sync.Mutex
	tok *Token
}

func (s *SavedTokens) current() (Token, error) {
	if s.tok != nil {
		return *s.tok, nil
	}

	t, err := s.Load()
	if err != nil {
		return t, err
	}

	s.tok = &t

	return t, nil
}

/*
Token returns the access token, refreshing it a minute before it expires.
*/
func (s *SavedTokens) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	t, err := s.current()
	s.mu.Unlock()

	if err != nil {
		return "", err
	}

	if !t.ExpiresAt.IsZero() && time.Until(t.ExpiresAt) < time.Minute && t.RefreshToken != "" {
		return s.Refresh(ctx)
	}

	return t.AccessToken, nil
}

/*
Refresh trades the refresh token for a new pair and saves it.
*/
func (s *SavedTokens) Refresh(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.current()
	if err != nil {
		return "", err
	}

	if t.RefreshToken == "" {
		return "", errors.New("the dimmi sign-in expired: run `dimmi login` again")
	}

	hc := s.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}

	meta, err := Discover(ctx, hc, t.BaseURL)
	if err != nil {
		return "", err
	}

	next, err := tokenRequest(ctx, hc, meta.TokenEndpoint, t.BaseURL, t.ClientID, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
		"client_id":     {t.ClientID},
	})
	if err != nil {
		return "", fmt.Errorf("%w: run `dimmi login` again", err)
	}

	if next.RefreshToken == "" {
		next.RefreshToken = t.RefreshToken
	}

	if err := s.Save(next); err != nil {
		return "", err
	}

	s.tok = &next

	return next.AccessToken, nil
}
