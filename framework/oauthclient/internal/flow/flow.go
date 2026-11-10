/*
|--------------------------------------------------------------------------
| Authorization Code Flow
|--------------------------------------------------------------------------
|
| Providers that follow the standard send the browser out with the same
| parameters and trade codes and refresh tokens back the same way, so the
| flow is written once. The URL only ever carries the S256 challenge of
| the PKCE verifier, and the verifier itself only travels on the
| server-to-server exchange. Every call goes out through the
| application's HTTP client, under the same connection checks as any
| other outbound call.
|
*/

package flow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"
)

/*
Client is the OAuth 2 client registered with one provider. Config holds
its credentials, endpoints and default scopes; its redirect URL is
ignored in favor of the attempt's. PKCE says whether the provider takes
a challenge, and ScopeSeparator joins scopes, a space when empty. HTTP
sends every call.
*/
type Client struct {
	Config         oauth2.Config
	PKCE           bool
	ScopeSeparator string
	HTTP           *httpclient.Client
}

/*
AuthURL returns the authorization URL for the attempt, asking for its
scopes or the defaults, with its extra parameters.
*/
func (c Client) AuthURL(attempt oauthclient.Attempt) string {
	cfg := c.Config
	cfg.RedirectURL = attempt.RedirectURL
	cfg.Scopes = nil

	scopes := attempt.Scopes
	if len(scopes) == 0 {
		scopes = c.Config.Scopes
	}

	var opts []oauth2.AuthCodeOption
	if len(scopes) > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("scope", strings.Join(scopes, c.separator())))
	}

	for key, value := range attempt.Extra {
		opts = append(opts, oauth2.SetAuthURLParam(key, value))
	}

	if c.PKCE {
		opts = append(opts, oauth2.S256ChallengeOption(attempt.Verifier))
	}

	return cfg.AuthCodeURL(attempt.State, opts...)
}

/*
Exchange trades the code for a grant, proving the attempt's verifier
when the provider takes PKCE. A refusal by the provider is ErrExchange;
failing to reach it or to read its answer is ErrProvider.
*/
func (c Client) Exchange(ctx context.Context, code string, attempt oauthclient.Attempt) (oauthclient.Grant, error) {
	cfg := c.Config
	cfg.RedirectURL = attempt.RedirectURL

	var opts []oauth2.AuthCodeOption
	if c.PKCE {
		opts = append(opts, oauth2.VerifierOption(attempt.Verifier))
	}

	token, err := cfg.Exchange(c.context(ctx), code, opts...)
	if err != nil {
		return oauthclient.Grant{}, classify(err, oauthclient.ErrExchange)
	}

	return c.grant(token), nil
}

/*
Refresh trades the refresh token for a fresh grant. A refusal by the
provider is ErrInvalidGrant.
*/
func (c Client) Refresh(ctx context.Context, refresh string) (oauthclient.Grant, error) {
	token, err := c.Config.TokenSource(c.context(ctx), &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		return oauthclient.Grant{}, classify(err, oauthclient.ErrInvalidGrant)
	}

	if token.RefreshToken == "" {
		token.RefreshToken = refresh
	}

	return c.grant(token), nil
}

/*
Revoke gives the token back at the revocation URL the standard
describes, authenticating as the client.
*/
func (c Client) Revoke(ctx context.Context, revokeURL, token string) error {
	if revokeURL == "" {
		return oauthclient.ErrUnsupported
	}

	form := url.Values{
		"token": {token}, "client_id": {c.Config.ClientID}, "client_secret": {c.Config.ClientSecret},
	}.Encode()

	resp, err := c.HTTP.New().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Post(ctx, revokeURL, form)
	if err != nil {
		return fmt.Errorf("%w: %w", oauthclient.ErrProvider, err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("%w: revoke answered %d", oauthclient.ErrProvider, resp.Status)
	}

	return nil
}

func (c Client) separator() string {
	if c.ScopeSeparator == "" {
		return " "
	}

	return c.ScopeSeparator
}

func (c Client) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.HTTP.StdClient())
}

/*
grant normalizes the token response.
*/
func (c Client) grant(token *oauth2.Token) oauthclient.Grant {
	grant := oauthclient.Grant{
		Access: &oauthclient.Token{Value: token.AccessToken, Expiry: token.Expiry},
		Raw:    map[string]any{"token_type": token.TokenType},
	}

	if token.RefreshToken != "" {
		grant.Refresh = &oauthclient.Token{Value: token.RefreshToken}
	}

	if scope, _ := token.Extra("scope").(string); scope != "" {
		grant.Scopes = splitScopes(scope)
	}

	return grant
}

/*
splitScopes reads a scope list joined by spaces or commas.
*/
func splitScopes(scope string) []string {
	return strings.FieldsFunc(scope, func(r rune) bool { return r == ' ' || r == ',' })
}

/*
classify maps an error from the token endpoint: a refusal by the
provider is the refusal error, anything else ErrProvider.
*/
func classify(err, refused error) error {
	var retrieve *oauth2.RetrieveError
	if errors.As(err, &retrieve) && retrieve.Response != nil && retrieve.Response.StatusCode < 500 {
		return fmt.Errorf("%w: %d %s", refused, retrieve.Response.StatusCode, retrieve.ErrorCode)
	}

	return fmt.Errorf("%w: %w", oauthclient.ErrProvider, err)
}
