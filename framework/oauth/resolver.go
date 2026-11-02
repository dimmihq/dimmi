/*
|--------------------------------------------------------------------------
| Access Token Resolver
|--------------------------------------------------------------------------
|
| The bearer guard hands every token with the access token prefix to this
| resolver, which decides what kind of caller it stands for by looking
| the grant up, never by anything the client claims. A first-party grant
| is the application's own app acting as the user, so it answers as
| GuardFirstParty with every ability and no tenant, and the user's role
| decides the rest. Any other grant answers as GuardThirdParty, bound to
| the tenant the user chose and holding only the scopes they approved.
|
| A grant remembers when the user last logged in and when they last
| confirmed their identity in the app, so recent-authentication checks
| work for app callers the way they do for browser sessions.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ErrGrantNotFound is returned by ConfirmGrant when no live grant has the
id.
*/
var ErrGrantNotFound = errors.New("oauth: grant not found")

/*
Resolver returns the bearer token resolver for the server's access
tokens. It answers only for tokens with the access token prefix. The
principal's Credential is the grant, which is what a user sees and
revokes. An unknown token is auth.ErrInvalidToken, and an expired or
revoked one token.ErrTokenExpired or token.ErrTokenRevoked.
*/
func (s *Server) Resolver() auth.TokenResolver {
	prefix := s.opts.Prefix + kindAccess

	return auth.TokenResolverFunc(func(r *http.Request, plain string) (auth.Principal, error) {
		if !strings.HasPrefix(plain, prefix) {
			return nil, auth.ErrNoCredentials
		}

		tok, err := s.access(r.Context(), plain)
		if err != nil {
			return nil, err
		}

		return principal(tok), nil
	})
}

/*
access returns the live access token with the plain value.
*/
func (s *Server) access(ctx context.Context, plain string) (tokenRecord, error) {
	if !s.wellFormed(plain, kindAccess) {
		return tokenRecord{}, auth.ErrInvalidToken
	}

	tok, err := findToken(ctx, s.db, digest(plain))
	if errors.Is(err, database.ErrNotFound) {
		return tokenRecord{}, auth.ErrInvalidToken
	}

	if err != nil {
		return tokenRecord{}, fmt.Errorf("find access token: %w", err)
	}

	switch {
	case !tok.revokedAt.IsZero() || !tok.grant.revokedAt.IsZero():
		return tokenRecord{}, token.ErrTokenRevoked
	case !s.clock.Now().Before(tok.expiresAt):
		return tokenRecord{}, token.ErrTokenExpired
	}

	return tok, nil
}

/*
principal returns the caller an access token stands for.
*/
func principal(tok tokenRecord) *auth.Identity {
	id := &auth.Identity{
		User:       tok.grant.userID,
		Via:        GuardThirdParty,
		Credential: tok.grant.id,
		TenantID:   tok.grant.tenantID,
		Granted:    slices.Clone(tok.scopes),
		Confirmed:  later(tok.grant.authTime, tok.grant.confirmedAt),
	}

	if tok.grant.firstParty {
		id.Via, id.TenantID, id.Granted = GuardFirstParty, "", []string{auth.AllAbilities}
	}

	return id
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}

	return a
}

/*
RevokeGrant ends the grant and every token issued under it, as when the
user disconnects an app or signs a device out. Revoking a grant twice
changes nothing.
*/
func (s *Server) RevokeGrant(ctx context.Context, id ids.UUID) error {
	return s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return revokeGrant(ctx, tx, id, s.clock.Now())
	})
}

/*
ConfirmGrant records that the user just proved their identity again
through the grant's app, such as by entering their password, so the
grant's tokens pass recent-authentication checks. It returns
ErrGrantNotFound when the grant is unknown or revoked.
*/
func (s *Server) ConfirmGrant(ctx context.Context, id ids.UUID) error {
	sql := "UPDATE oauth_grants SET confirmed_at = $2 WHERE id = $1 AND revoked_at IS NULL"

	tag, err := s.db.Exec(ctx, sql, id, s.clock.Now())
	if err != nil {
		return fmt.Errorf("confirm grant: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrGrantNotFound
	}

	return nil
}
