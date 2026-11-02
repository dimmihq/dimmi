/*
|--------------------------------------------------------------------------
| Token Store
|--------------------------------------------------------------------------
|
| A token's secret is shown once, when it is made, and never stored. The
| store keeps a SHA-256 digest to find the token by, a short hint the
| owner recognizes it by, and what it may do. A revoked token stays on
| record, so its owner can still see when it was used and that it ended.
|
*/

package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ErrNotFound means no live token matches the digest or id.
*/
var ErrNotFound = errors.New("token: not found")

/*
DefaultTable is the table the bundled migration creates.
*/
const DefaultTable = "personal_access_tokens"

/*
columns lists the stored columns in the order every query reads them.
*/
const columns = "id, owner_id, tenant_id, name, hint, abilities, expires_at, last_used_at, last_used_ip, " +
	"revoked_at, created_at"

/*
Store keeps tokens.
*/
type Store interface {
	/*
		Insert stores a new token under the digest of its secret.
	*/
	Insert(ctx context.Context, t Token, hash []byte) error

	/*
		FindByHash returns the token whose secret has the digest, or
		ErrNotFound.
	*/
	FindByHash(ctx context.Context, hash []byte) (Token, error)

	/*
		Touch records that the token was used at the time from the address.
	*/
	Touch(ctx context.Context, id ids.UUID, at time.Time, ip string) error

	/*
		Revoke marks the token revoked at the time, or returns ErrNotFound
		when there is no such token or it is already revoked.
	*/
	Revoke(ctx context.Context, id ids.UUID, at time.Time) error

	/*
		ForOwner returns the owner's tokens, revoked ones included, newest
		first.
	*/
	ForOwner(ctx context.Context, ownerID ids.UUID) ([]Token, error)
}

/*
PostgresStore keeps tokens in a PostgreSQL table.
*/
type PostgresStore struct {
	db    *database.DB
	table string
}

var _ Store = (*PostgresStore)(nil)

/*
NewPostgresStore returns a store on the table, or on DefaultTable when
the name is empty.
*/
func NewPostgresStore(db *database.DB, table string) *PostgresStore {
	if table == "" {
		table = DefaultTable
	}

	return &PostgresStore{db: db, table: pgx.Identifier(strings.Split(table, ".")).Sanitize()}
}

/*
Insert stores a new token.
*/
func (s *PostgresStore) Insert(ctx context.Context, t Token, hash []byte) error {
	sql := "INSERT INTO " + s.table + " (id, owner_id, tenant_id, name, hint, abilities, expires_at, " +
		"created_at, token_hash) VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9)"

	_, err := s.db.Exec(ctx, sql, t.ID, t.OwnerID, t.TenantID, t.Name, t.Hint, t.Abilities,
		nullTime(t.ExpiresAt), t.CreatedAt, hash)
	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}

	return nil
}

/*
FindByHash returns the token whose secret has the digest.
*/
func (s *PostgresStore) FindByHash(ctx context.Context, hash []byte) (Token, error) {
	t, err := scanToken(s.db.QueryRow(ctx, "SELECT "+columns+" FROM "+s.table+" WHERE token_hash = $1", hash))
	if errors.Is(err, database.ErrNotFound) {
		return Token{}, ErrNotFound
	}

	if err != nil {
		return Token{}, fmt.Errorf("find token: %w", err)
	}

	return t, nil
}

/*
Touch records the token's last use.
*/
func (s *PostgresStore) Touch(ctx context.Context, id ids.UUID, at time.Time, ip string) error {
	sql := "UPDATE " + s.table + " SET last_used_at = $2, last_used_ip = NULLIF($3, '') WHERE id = $1"
	if _, err := s.db.Exec(ctx, sql, id, at, ip); err != nil {
		return fmt.Errorf("touch token: %w", err)
	}

	return nil
}

/*
Revoke marks a live token revoked.
*/
func (s *PostgresStore) Revoke(ctx context.Context, id ids.UUID, at time.Time) error {
	sql := "UPDATE " + s.table + " SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL"

	tag, err := s.db.Exec(ctx, sql, id, at)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

/*
ForOwner returns the owner's tokens, newest first.
*/
func (s *PostgresStore) ForOwner(ctx context.Context, ownerID ids.UUID) ([]Token, error) {
	sql := "SELECT " + columns + " FROM " + s.table + " WHERE owner_id = $1 ORDER BY created_at DESC, id DESC"

	rows, err := s.db.Query(ctx, sql, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []Token

	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, fmt.Errorf("read token: %w", err)
		}

		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	return tokens, nil
}

func scanToken(row pgx.Row) (Token, error) {
	var (
		t                            Token
		tenant, lastUsedIP           *string
		expiresAt, lastUsed, revoked *time.Time
	)

	err := row.Scan(&t.ID, &t.OwnerID, &tenant, &t.Name, &t.Hint, &t.Abilities, &expiresAt, &lastUsed,
		&lastUsedIP, &revoked, &t.CreatedAt)
	if err != nil {
		return Token{}, err
	}

	t.TenantID, t.LastUsedIP = deref(tenant), deref(lastUsedIP)
	t.ExpiresAt, t.LastUsedAt, t.RevokedAt = deref(expiresAt), deref(lastUsed), deref(revoked)

	return t, nil
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
