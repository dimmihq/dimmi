/*
|--------------------------------------------------------------------------
| Token Endpoint
|--------------------------------------------------------------------------
|
| The token endpoint is where codes and refresh tokens are spent. A
| client proves who it is first, with its secret when it has one, and may
| use only the grants it registered. Every spend runs in a transaction
| that locks what it spends, so the same code or refresh token cannot be
| redeemed twice by racing requests. A refusal that revokes a grant, such
| as a replayed code, still commits the revocation.
|
*/

package oauth

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
tokenResponse is a successful token response.
*/
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

/*
issuance is what a new pair of tokens is issued from: the client, its
grant, the scopes of the access token and the refresh token the new one
replaces, if any.
*/
type issuance struct {
	client Client
	grant  grant
	scopes []string
	parent ids.UUID
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	res, err := s.token(w, r)
	if err != nil {
		s.writeError(w, r, err)

		return
	}

	noStore(w)
	http.WriteJSON(w, http.StatusOK, res)
}

/*
token authenticates the client and runs the grant it asks for.
*/
func (s *Server) token(w http.ResponseWriter, r *http.Request) (*tokenResponse, error) {
	if err := parseForm(w, r); err != nil {
		return nil, err
	}

	client, err := s.authenticate(r)
	if err != nil {
		return nil, err
	}

	grantType := r.PostForm.Get("grant_type")

	switch {
	case !slices.Contains(grantTypes(), grantType):
		return nil, refuse(CodeUnsupportedGrantType, "grant_type must be authorization_code or refresh_token.")
	case !slices.Contains(client.GrantTypes, grantType):
		return nil, refuse(CodeUnauthorizedClient, "The client may not use this grant type.")
	case grantType == GrantRefreshToken:
		return s.refresh(r.Context(), client, r.PostForm)
	}

	return s.exchange(r.Context(), client, r.PostForm)
}

/*
authenticate returns the client the request comes from. Credentials come
from HTTP Basic or from the form; a confidential client must present
its secret.
*/
func (s *Server) authenticate(r *http.Request) (Client, error) {
	id, secret, basic := r.BasicAuth()
	if basic {
		id, secret = unescape(id), unescape(secret)
	} else {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}

	client, err := s.findClient(r.Context(), id)
	if err != nil {
		return Client{}, err
	}

	if client.Confidential && !hashing.Equal(digest(secret), client.secretHash) {
		return Client{}, errInvalidClient()
	}

	return client, nil
}

/*
unescape decodes a Basic credential, which clients form-encode before
encoding the pair.
*/
func unescape(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}

	return decoded
}

/*
issue stores a new access token, and a refresh token when the client
may refresh, and returns the response that hands them over.
*/
func (s *Server) issue(ctx context.Context, tx *database.Tx, in issuance) (*tokenResponse, error) {
	now := s.clock.Now()

	ttl := s.opts.AccessTokenTTL
	if in.grant.firstParty {
		ttl = s.opts.FirstPartyAccessTokenTTL
	}

	access := newToken{
		kind: tokenAccess, plain: s.newSecret(kindAccess), grantID: in.grant.id,
		scopes: in.scopes, expiresAt: now.Add(ttl),
	}
	if err := insertToken(ctx, tx, access, now); err != nil {
		return nil, err
	}

	res := &tokenResponse{
		AccessToken: access.plain, TokenType: "Bearer", ExpiresIn: int64(ttl / time.Second),
		Scope: strings.Join(in.scopes, " "),
	}

	if !slices.Contains(in.client.GrantTypes, GrantRefreshToken) {
		return res, nil
	}

	refresh := newToken{
		kind: tokenRefresh, plain: s.newSecret(kindRefresh), grantID: in.grant.id,
		parentID: in.parent, scopes: in.grant.scopes, expiresAt: now.Add(s.opts.RefreshTokenTTL),
	}
	if err := insertToken(ctx, tx, refresh, now); err != nil {
		return nil, err
	}

	res.RefreshToken = refresh.plain

	return res, nil
}

/*
transact runs the spend in a transaction. A refusal the spend returns
still commits what it wrote first, such as a revoked grant.
*/
func (s *Server) transact(ctx context.Context,
	spend func(ctx context.Context, tx *database.Tx) (*tokenResponse, error),
) (*tokenResponse, error) {
	var (
		res     *tokenResponse
		refusal *Error
	)

	err := s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err := spend(ctx, tx)
		if e, ok := errors.AsType[*Error](err); ok {
			refusal = e

			return nil
		}

		res = out

		return err
	})

	switch {
	case err != nil:
		return nil, err
	case refusal != nil:
		return nil, refusal
	}

	return res, nil
}
