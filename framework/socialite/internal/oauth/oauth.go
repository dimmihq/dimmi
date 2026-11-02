/*
|--------------------------------------------------------------------------
| Authorization Code Flow
|--------------------------------------------------------------------------
|
| Every driver sends the browser out with the same parameters and trades
| the code back the same way, so the flow is written once. The URL only
| ever carries the S256 challenge of the PKCE verifier, and the verifier
| itself only travels on the server-to-server exchange. The exchange
| goes out through the application's HTTP client, so the provider is
| reached under the same connection checks as any other outbound call.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/socialite"
)

/*
Client is the OAuth 2 client registered with one provider. Config holds
its credentials, endpoints and scopes; its redirect URL is ignored in
favor of the attempt's. HTTP sends the exchange.
*/
type Client struct {
	Config oauth2.Config
	HTTP   *httpclient.Client
}

/*
AuthURL returns the authorization URL for the attempt, with any extra
parameters the provider takes.
*/
func (c Client) AuthURL(attempt socialite.Attempt, params ...oauth2.AuthCodeOption) string {
	cfg := c.Config
	cfg.RedirectURL = attempt.RedirectURL

	return cfg.AuthCodeURL(attempt.State, append(params, oauth2.S256ChallengeOption(attempt.Verifier))...)
}

/*
Exchange trades the code for tokens, proving the attempt's verifier. A
refusal by the provider is ErrExchange; failing to reach it or to read
its answer is ErrProvider.
*/
func (c Client) Exchange(ctx context.Context, code string, attempt socialite.Attempt) (socialite.Token, error) {
	cfg := c.Config
	cfg.RedirectURL = attempt.RedirectURL
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.HTTP.StdClient())

	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(attempt.Verifier))
	if err != nil {
		return socialite.Token{}, exchangeError(err)
	}

	idToken, _ := token.Extra("id_token").(string)

	return socialite.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		IDToken:      idToken,
	}, nil
}

func exchangeError(err error) error {
	var refused *oauth2.RetrieveError
	if errors.As(err, &refused) && refused.Response != nil && refused.Response.StatusCode < 500 {
		return fmt.Errorf("%w: %d %s", socialite.ErrExchange, refused.Response.StatusCode, refused.ErrorCode)
	}

	return fmt.Errorf("%w: %w", socialite.ErrProvider, err)
}
