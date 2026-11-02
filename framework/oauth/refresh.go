/*
|--------------------------------------------------------------------------
| Refresh Token Rotation
|--------------------------------------------------------------------------
|
| Every refresh spends the refresh token and hands out a new one, so a
| refresh token works exactly once. A spent token that comes back means
| two parties hold it, and the server cannot tell the app from the thief,
| so the whole grant is revoked: every access and refresh token in the
| family stops working and the user has to approve the client again. A
| refresh may narrow the scopes of the new access token, never widen
| them, and leaves the grant as it was.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
refresh spends a refresh token for a new pair of tokens.
*/
func (s *Server) refresh(ctx context.Context, client Client, form url.Values) (*tokenResponse, error) {
	plain := form.Get("refresh_token")
	if !s.wellFormed(plain, kindRefresh) {
		return nil, refuse(CodeInvalidGrant, "The refresh token is invalid.")
	}

	asked := parseScopes(form.Get("scope"))

	return s.transact(ctx, func(ctx context.Context, tx *database.Tx) (*tokenResponse, error) {
		tok, err := lockToken(ctx, tx, digest(plain))
		if errors.Is(err, database.ErrNotFound) {
			return nil, refuse(CodeInvalidGrant, "The refresh token is invalid.")
		}

		if err != nil {
			return nil, fmt.Errorf("find refresh token: %w", err)
		}

		return s.rotate(ctx, tx, client, refreshing{token: tok, scopes: asked})
	})
}

/*
refreshing is a refresh token being spent and the scopes asked for.
*/
type refreshing struct {
	token  tokenRecord
	scopes []string
}

/*
rotate checks the refresh token, spends it and issues its successors.
*/
func (s *Server) rotate(ctx context.Context, tx *database.Tx, client Client, in refreshing) (*tokenResponse, error) {
	tok, now := in.token, s.clock.Now()

	switch {
	case tok.grant.clientID != client.ID:
		return nil, refuse(CodeInvalidGrant, "The refresh token was issued to another client.")
	case !tok.revokedAt.IsZero() || !tok.grant.revokedAt.IsZero():
		return nil, refuse(CodeInvalidGrant, "The refresh token has been revoked.")
	case !tok.usedAt.IsZero():
		if err := revokeGrant(ctx, tx, tok.grant.id, now); err != nil {
			return nil, err
		}

		return nil, refuse(CodeInvalidGrant, "The refresh token was already used; its grant is revoked.")
	case !now.Before(tok.expiresAt):
		return nil, refuse(CodeInvalidGrant, "The refresh token has expired.")
	case !subset(in.scopes, tok.grant.scopes):
		return nil, refuse(CodeInvalidScope, "A refresh may not add scopes the grant does not hold.")
	}

	scopes := in.scopes
	if len(scopes) == 0 {
		scopes = tok.grant.scopes
	}

	if err := markTokenUsed(ctx, tx, tok.id, now); err != nil {
		return nil, err
	}

	return s.issue(ctx, tx, issuance{client: client, grant: tok.grant, scopes: scopes, parent: tok.id})
}
