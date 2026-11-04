/*
|--------------------------------------------------------------------------
| Account Authorization
|--------------------------------------------------------------------------
|
| Connecting a mailbox asks its owner for lasting access through OAuth,
| with a proof key so a stolen authorization code is worthless. Sign-in
| with a provider is a different thing: that needs no offline token and no
| mail scopes. Refreshing may hand back a new refresh token, and some
| providers invalidate the old one at once, so the caller must keep
| whichever comes back.
|
| Why a grant failed matters to the person reading the result, so those
| failures are told apart here: revoked, or blocked by an administrator.
|
*/

package accountauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
Tokens are what a provider granted. Scopes is what it says it granted,
empty when it does not say.
*/
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Scopes       []string  `json:"scopes"`
}

/*
Config names one provider's OAuth client. Params are added to the
authorization address, such as asking for an offline token.
*/
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
	Params       map[string]string
}

/*
Client authorizes accounts of one provider. HTTP carries every call to
the provider.
*/
type Client struct {
	cfg  oauth2.Config
	http *httpclient.Client
	opts []oauth2.AuthCodeOption
}

/*
New returns a client for the provider.
*/
func New(cfg Config, httpClient *httpclient.Client) *Client {
	opts := make([]oauth2.AuthCodeOption, 0, len(cfg.Params))
	for name, value := range cfg.Params {
		opts = append(opts, oauth2.SetAuthURLParam(name, value))
	}

	return &Client{
		http: httpClient,
		opts: opts,
		cfg: oauth2.Config{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, Scopes: cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL, AuthStyle: oauth2.AuthStyleInParams,
			},
		},
	}
}

/*
NewVerifier returns a fresh proof key to keep until the code comes back.
*/
func NewVerifier() string {
	return oauth2.GenerateVerifier()
}

/*
AuthorizationURL returns where to send the person, with the state and the
proof of the verifier. Scopes extend the client's for this one request.
*/
func (c *Client) AuthorizationURL(state, verifier string, scopes ...string) string {
	cfg := c.cfg
	cfg.Scopes = append(append([]string{}, cfg.Scopes...), scopes...)

	opts := append(append([]oauth2.AuthCodeOption{}, c.opts...), oauth2.S256ChallengeOption(verifier))

	return cfg.AuthCodeURL(state, opts...)
}

/*
Exchange trades the code for tokens.
*/
func (c *Client) Exchange(ctx context.Context, code, verifier string) (Tokens, error) {
	token, err := c.cfg.Exchange(c.context(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Tokens{}, mapError(err)
	}

	return tokensOf(token), nil
}

/*
Refresh returns fresh tokens. The refresh token stays the given one unless
the provider rotated it.
*/
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	source := c.cfg.TokenSource(c.context(ctx), &oauth2.Token{RefreshToken: refreshToken})

	token, err := source.Token()
	if err != nil {
		return Tokens{}, mapError(err)
	}

	return tokensOf(token), nil
}

func (c *Client) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.http.StdClient())
}

func tokensOf(token *oauth2.Token) Tokens {
	tokens := Tokens{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, Expiry: token.Expiry}
	if scope, ok := token.Extra("scope").(string); ok {
		tokens.Scopes = strings.Fields(scope)
	}

	return tokens
}

/*
blockedCodes are the provider answers that mean an administrator stopped
the application: Google's policy error and Microsoft's missing consent.
*/
func blockedCodes() []string {
	return []string{"admin_policy_enforced", "AADSTS65004", "AADSTS65001", "AADSTS90094"}
}

/*
revokedCodes are Microsoft's detailed answers for a grant that ended,
beside the standard invalid_grant.
*/
func revokedCodes() []string {
	return []string{"AADSTS70008", "AADSTS50173", "AADSTS700082", "invalid_grant"}
}

func mapError(err error) error {
	var retrieve *oauth2.RetrieveError
	if !errors.As(err, &retrieve) {
		return fmt.Errorf("oauth token: %w", err)
	}

	text := retrieve.ErrorCode + " " + retrieve.ErrorDescription + " " + string(retrieve.Body)

	for _, code := range blockedCodes() {
		if strings.Contains(text, code) {
			return fmt.Errorf("%w: %s", mailbox.ErrAdminBlocked, retrieve.ErrorCode)
		}
	}

	for _, code := range revokedCodes() {
		if strings.Contains(text, code) {
			return fmt.Errorf("%w: %s", mailbox.ErrGrantRevoked, retrieve.ErrorCode)
		}
	}

	return fmt.Errorf("oauth token: status %d: %s", retrieve.Response.StatusCode, retrieve.ErrorCode)
}
