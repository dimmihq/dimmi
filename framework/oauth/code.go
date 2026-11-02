/*
|--------------------------------------------------------------------------
| Code Exchange
|--------------------------------------------------------------------------
|
| An authorization code is spent once, within a minute, by the client it
| was issued to, from the redirect URI it was sent to, with the verifier
| of its PKCE challenge. Redeeming it creates the grant. A code that
| comes back after it was spent has leaked, and nobody can tell whether
| the first or the second request was the thief, so the grant it created
| is revoked along with every token issued under it.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
codeRecord is a stored authorization code.
*/
type codeRecord struct {
	id          ids.UUID
	clientID    string
	userID      ids.UUID
	tenantID    string
	scopes      []string
	redirectURI string
	challenge   string
	authTime    time.Time
	grantID     ids.UUID
	expiresAt   time.Time
	usedAt      time.Time
}

/*
exchange redeems an authorization code for tokens.
*/
func (s *Server) exchange(ctx context.Context, client Client, form url.Values) (*tokenResponse, error) {
	plain := form.Get("code")
	if !s.wellFormed(plain, kindCode) {
		return nil, refuse(CodeInvalidGrant, "The authorization code is invalid.")
	}

	return s.transact(ctx, func(ctx context.Context, tx *database.Tx) (*tokenResponse, error) {
		code, err := lockCode(ctx, tx, digest(plain))
		if errors.Is(err, database.ErrNotFound) {
			return nil, refuse(CodeInvalidGrant, "The authorization code is invalid.")
		}

		if err != nil {
			return nil, fmt.Errorf("find authorization code: %w", err)
		}

		if code.clientID != client.ID {
			return nil, refuse(CodeInvalidGrant, "The authorization code was issued to another client.")
		}

		if !code.usedAt.IsZero() {
			return nil, s.refuseReplay(ctx, tx, code)
		}

		if err := checkCode(code, form, s.clock.Now()); err != nil {
			return nil, err
		}

		return s.redeem(ctx, tx, client, code)
	})
}

/*
checkCode refuses a code that expired or whose redirect URI or PKCE
verifier does not match the request.
*/
func checkCode(code codeRecord, form url.Values, now time.Time) error {
	switch {
	case !now.Before(code.expiresAt):
		return refuse(CodeInvalidGrant, "The authorization code has expired.")
	case form.Get("redirect_uri") != code.redirectURI:
		return refuse(CodeInvalidGrant, "The redirect_uri does not match the authorization request.")
	case !verifyPKCE(form.Get("code_verifier"), code.challenge):
		return refuse(CodeInvalidGrant, "The code_verifier does not match the code_challenge.")
	}

	return nil
}

/*
refuseReplay revokes the grant a spent code created and refuses the
request.
*/
func (s *Server) refuseReplay(ctx context.Context, tx *database.Tx, code codeRecord) error {
	if err := revokeGrant(ctx, tx, code.grantID, s.clock.Now()); err != nil {
		return err
	}

	return refuse(CodeInvalidGrant, "The authorization code was already used; its grant is revoked.")
}

/*
redeem spends the code, creates its grant and issues the first tokens.
*/
func (s *Server) redeem(ctx context.Context, tx *database.Tx, client Client, code codeRecord) (*tokenResponse,
	error,
) {
	now := s.clock.Now()
	g := grant{
		id:         ids.NewV7At(now),
		clientID:   client.ID,
		userID:     code.userID,
		tenantID:   code.tenantID,
		firstParty: client.FirstParty,
		scopes:     code.scopes,
		authTime:   code.authTime,
	}

	if err := insertGrant(ctx, tx, g, now); err != nil {
		return nil, err
	}

	sql := "UPDATE oauth_authorization_codes SET used_at = $2, grant_id = $3 WHERE id = $1"
	if _, err := tx.Exec(ctx, sql, code.id, now, g.id); err != nil {
		return nil, fmt.Errorf("spend authorization code: %w", err)
	}

	return s.issue(ctx, tx, issuance{client: client, grant: g, scopes: g.scopes})
}

/*
lockCode returns the code with the digest, locked until the transaction
ends, or database.ErrNotFound.
*/
func lockCode(ctx context.Context, tx *database.Tx, hash []byte) (codeRecord, error) {
	sql := "SELECT id, client_id, user_id, tenant_id, scopes, redirect_uri, code_challenge, auth_time, grant_id, " +
		"expires_at, used_at FROM oauth_authorization_codes WHERE code_hash = $1 FOR UPDATE"

	return scanCode(tx.QueryRow(ctx, sql, hash))
}

func scanCode(row pgx.Row) (codeRecord, error) {
	var (
		c              codeRecord
		tenant         *string
		authTime, used *time.Time
		grantID        *ids.UUID
	)

	err := row.Scan(&c.id, &c.clientID, &c.userID, &tenant, &c.scopes, &c.redirectURI, &c.challenge, &authTime,
		&grantID, &c.expiresAt, &used)
	if err != nil {
		return codeRecord{}, err
	}

	c.tenantID, c.authTime, c.grantID, c.usedAt = deref(tenant), deref(authTime), deref(grantID), deref(used)

	return c, nil
}
