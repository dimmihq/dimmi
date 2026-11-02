/*
|--------------------------------------------------------------------------
| Grant and Token Storage
|--------------------------------------------------------------------------
|
| A grant is one approval: a user letting one client act for them, with
| the scopes and tenant they chose. Every token descends from a grant,
| and revoking the grant ends every token in its family at once, which
| is what a stolen refresh token or a replayed code calls for. Tokens are
| found by the digest of their value, together with their grant, in one
| query, because every check needs both.
|
*/

package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The kinds of token rows.
*/
const (
	tokenAccess  = "access"
	tokenRefresh = "refresh"
)

/*
selectToken reads a token and its grant by the token's digest.
*/
const selectToken = "SELECT t.id, t.kind, t.scopes, t.expires_at, t.used_at, t.revoked_at, g.id, g.client_id, " +
	"g.user_id, g.tenant_id, g.first_party, g.scopes, g.auth_time, g.confirmed_at, g.revoked_at " +
	"FROM oauth_tokens t JOIN oauth_grants g ON g.id = t.grant_id WHERE t.token_hash = $1"

/*
grant is one approval of one client by one user.
*/
type grant struct {
	id          ids.UUID
	clientID    string
	userID      ids.UUID
	tenantID    string
	firstParty  bool
	scopes      []string
	authTime    time.Time
	confirmedAt time.Time
	revokedAt   time.Time
}

/*
tokenRecord is a stored token with its grant.
*/
type tokenRecord struct {
	id        ids.UUID
	kind      string
	scopes    []string
	expiresAt time.Time
	usedAt    time.Time
	revokedAt time.Time
	grant     grant
}

/*
newToken is a token row to insert.
*/
type newToken struct {
	kind      string
	plain     string
	grantID   ids.UUID
	parentID  ids.UUID
	scopes    []string
	expiresAt time.Time
}

func insertGrant(ctx context.Context, q database.Querier, g grant, at time.Time) error {
	sql := "INSERT INTO oauth_grants (id, client_id, user_id, tenant_id, first_party, scopes, auth_time, created_at) " +
		"VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)"

	_, err := q.Exec(ctx, sql, g.id, g.clientID, g.userID, g.tenantID, g.firstParty, g.scopes, nullTime(g.authTime), at)
	if err != nil {
		return fmt.Errorf("insert grant: %w", err)
	}

	return nil
}

func insertToken(ctx context.Context, q database.Querier, t newToken, at time.Time) error {
	var parent *ids.UUID
	if t.parentID != ids.Nil() {
		parent = &t.parentID
	}

	sql := "INSERT INTO oauth_tokens (id, grant_id, kind, token_hash, parent_id, scopes, expires_at, created_at) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"

	_, err := q.Exec(ctx, sql, ids.NewV7At(at), t.grantID, t.kind, digest(t.plain), parent, t.scopes, t.expiresAt, at)
	if err != nil {
		return fmt.Errorf("insert %s token: %w", t.kind, err)
	}

	return nil
}

/*
findToken returns the token with the digest and its grant, or
database.ErrNotFound.
*/
func findToken(ctx context.Context, q database.Querier, hash []byte) (tokenRecord, error) {
	return scanToken(q.QueryRow(ctx, selectToken, hash))
}

/*
lockToken is findToken that also locks the token row until the
transaction ends, so two requests cannot both spend one refresh token.
*/
func lockToken(ctx context.Context, tx *database.Tx, hash []byte) (tokenRecord, error) {
	return scanToken(tx.QueryRow(ctx, selectToken+" FOR UPDATE OF t", hash))
}

func scanToken(row pgx.Row) (tokenRecord, error) {
	var (
		t                                    tokenRecord
		tenant                               *string
		used, revoked, auth, confirmed, gone *time.Time
	)

	err := row.Scan(&t.id, &t.kind, &t.scopes, &t.expiresAt, &used, &revoked, &t.grant.id, &t.grant.clientID,
		&t.grant.userID, &tenant, &t.grant.firstParty, &t.grant.scopes, &auth, &confirmed, &gone)
	if err != nil {
		return tokenRecord{}, err
	}

	t.usedAt, t.revokedAt, t.grant.tenantID = deref(used), deref(revoked), deref(tenant)
	t.grant.authTime, t.grant.confirmedAt, t.grant.revokedAt = deref(auth), deref(confirmed), deref(gone)

	return t, nil
}

func markTokenUsed(ctx context.Context, tx *database.Tx, id ids.UUID, at time.Time) error {
	if _, err := tx.Exec(ctx, "UPDATE oauth_tokens SET used_at = $2 WHERE id = $1", id, at); err != nil {
		return fmt.Errorf("spend refresh token: %w", err)
	}

	return nil
}

func revokeToken(ctx context.Context, q database.Querier, id ids.UUID, at time.Time) error {
	sql := "UPDATE oauth_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL"
	if _, err := q.Exec(ctx, sql, id, at); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

/*
revokeGrant ends the grant and every token issued under it. Revoking a
grant twice changes nothing.
*/
func revokeGrant(ctx context.Context, q database.Querier, id ids.UUID, at time.Time) error {
	sql := "UPDATE oauth_grants SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL"
	if _, err := q.Exec(ctx, sql, id, at); err != nil {
		return fmt.Errorf("revoke grant: %w", err)
	}

	sql = "UPDATE oauth_tokens SET revoked_at = $2 WHERE grant_id = $1 AND revoked_at IS NULL"
	if _, err := q.Exec(ctx, sql, id, at); err != nil {
		return fmt.Errorf("revoke grant tokens: %w", err)
	}

	return nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}
