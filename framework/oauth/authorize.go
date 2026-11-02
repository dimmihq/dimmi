/*
|--------------------------------------------------------------------------
| Authorization Requests
|--------------------------------------------------------------------------
|
| The application serves the authorization page: it logs the user in,
| shows the consent screen and records the answer. The server checks the
| request first, in an order that never redirects to an address it has
| not matched against the client's registration, and turns an approval
| into a single-use code bound to everything the token request must
| prove: the client, the redirect URI, the PKCE challenge, the user, the
| tenant, the scopes and when the user last logged in.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ErrInvalidApproval is returned by Approve for an approval the request
does not allow: no user, scopes that were never asked for, or a tenant
on a first-party grant.
*/
var ErrInvalidApproval = errors.New("oauth: invalid approval")

/*
AuthorizationRequest is an authorization request that passed every
check, ready for the consent screen.

Client is the client asking, RedirectURI where the answer goes, Scopes
what it asks for (every scope it registered when it named none), State
the client's value sent back untouched and CodeChallenge the PKCE S256
challenge the code is bound to.
*/
type AuthorizationRequest struct {
	Client        Client
	RedirectURI   string
	Scopes        []string
	State         string
	CodeChallenge string
	askedRedirect string
}

/*
Approval is the user's answer to an authorization request.

User is who approved and Tenant the tenant the grant is bound to, which
must be empty for a first-party client. Scopes are the scopes approved,
a subset of those asked for; nil approves all of them. AuthTime is when
the user last logged in fully.
*/
type Approval struct {
	User     ids.UUID
	Tenant   string
	Scopes   []string
	AuthTime time.Time
}

/*
ValidateAuthorization checks the query of an authorization request. A
refusal is an Error: with a RedirectURL when the client should hear of
it, without one when the application must show an error page instead.
The application checks the same query again when the consent form comes
back, rather than trusting a copy the browser could have changed.
*/
func (s *Server) ValidateAuthorization(ctx context.Context, query url.Values) (*AuthorizationRequest, error) {
	if len(query["client_id"]) > 1 || len(query["redirect_uri"]) > 1 {
		return nil, refuse(CodeInvalidRequest, "client_id and redirect_uri may appear only once.")
	}

	client, err := s.findClient(ctx, query.Get("client_id"))
	if err != nil {
		return nil, err
	}

	redirect, ok := matchRedirect(client.RedirectURIs, query.Get("redirect_uri"))
	if !ok {
		return nil, refuse(CodeInvalidRedirectURI, "The redirect_uri is not registered for this client.")
	}

	req := &AuthorizationRequest{
		Client:        client,
		RedirectURI:   redirect,
		State:         query.Get("state"),
		CodeChallenge: query.Get("code_challenge"),
		askedRedirect: query.Get("redirect_uri"),
	}

	scopes, err := s.checkAuthorization(req, query)
	if err != nil {
		return nil, err
	}

	req.Scopes = scopes

	return req, nil
}

/*
checkAuthorization checks the parameters that come after the redirect
URI and returns the scopes asked for. Its refusals redirect.
*/
func (s *Server) checkAuthorization(req *AuthorizationRequest, query url.Values) ([]string, error) {
	repeated := slices.ContainsFunc([]string{
		"response_type", "scope", "state", "code_challenge",
		"code_challenge_method",
	}, func(name string) bool { return len(query[name]) > 1 })

	switch {
	case repeated:
		return nil, s.redirectError(req, CodeInvalidRequest, "Parameters may appear only once.")
	case query.Get("response_type") != "code":
		return nil, s.redirectError(req, CodeUnsupportedResponseType, "response_type must be code.")
	case query.Get("code_challenge_method") != methodS256 || !pkceValue(req.CodeChallenge):
		return nil, s.redirectError(req, CodeInvalidRequest, "PKCE with code_challenge_method S256 is required.")
	}

	scopes := parseScopes(query.Get("scope"))
	if len(scopes) == 0 {
		return slices.Clone(req.Client.Scopes), nil
	}

	if !subset(scopes, req.Client.Scopes) {
		return nil, s.redirectError(req, CodeInvalidScope, "The client may not ask for these scopes.")
	}

	return scopes, nil
}

/*
Approve issues an authorization code for the approved request and
returns the URL to send the user to, which carries the code, the state
and the issuer.
*/
func (s *Server) Approve(ctx context.Context, req *AuthorizationRequest, a Approval) (string, error) {
	scopes := a.Scopes
	if scopes == nil {
		scopes = req.Scopes
	}

	switch {
	case a.User == ids.Nil():
		return "", fmt.Errorf("%w: no user", ErrInvalidApproval)
	case !subset(scopes, req.Scopes):
		return "", fmt.Errorf("%w: scopes %v were not asked for", ErrInvalidApproval, scopes)
	case req.Client.FirstParty && a.Tenant != "":
		return "", fmt.Errorf("%w: a first-party grant has no tenant", ErrInvalidApproval)
	}

	plain := s.newSecret(kindCode)
	now := s.clock.Now()
	sql := "INSERT INTO oauth_authorization_codes (id, code_hash, client_id, user_id, tenant_id, scopes, " +
		"redirect_uri, code_challenge, auth_time, expires_at, created_at) " +
		"VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $11)"

	_, err := s.db.Exec(ctx, sql, ids.NewV7At(now), digest(plain), req.Client.ID, a.User, a.Tenant,
		slices.Clone(scopes), req.askedRedirect, req.CodeChallenge, nullTime(a.AuthTime), now.Add(s.opts.CodeTTL), now)
	if err != nil {
		return "", fmt.Errorf("insert authorization code: %w", err)
	}

	return s.redirectTo(req, url.Values{"code": {plain}}), nil
}

/*
Deny returns the URL that tells the client the user refused.
*/
func (s *Server) Deny(req *AuthorizationRequest) string {
	return s.redirectTo(req, url.Values{"error": {CodeAccessDenied}, "error_description": {"The user denied access."}})
}

/*
redirectError returns a refusal that redirects to the request's
redirect URI.
*/
func (s *Server) redirectError(req *AuthorizationRequest, code, description string) *Error {
	refusal := refuse(code, description)
	refusal.redirect = s.redirectTo(req, url.Values{"error": {code}, "error_description": {description}})

	return refusal
}

/*
redirectTo returns the request's redirect URI with the parameters, the
state and the issuer added to its query.
*/
func (s *Server) redirectTo(req *AuthorizationRequest, params url.Values) string {
	u, err := url.Parse(req.RedirectURI)
	if err != nil {
		return ""
	}

	query := u.Query()
	maps.Copy(query, params)

	if req.State != "" {
		query.Set("state", req.State)
	}

	query.Set("iss", s.opts.Issuer)
	u.RawQuery = query.Encode()

	return u.String()
}
