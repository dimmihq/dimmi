/*
|--------------------------------------------------------------------------
| PostgreSQL Session Store
|--------------------------------------------------------------------------
|
| Sessions live in PostgreSQL beside the users they belong to, so signing
| a user out everywhere is one statement and survives any cache being
| flushed. The table name comes from configuration: an application whose
| sessions table carries more columns points the store at it, as long as
| it has the columns this store reads and writes.
|
*/

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
DefaultTable is the table the bundled migration creates.
*/
const DefaultTable = "sessions"

/*
columns lists the stored columns in the order every query reads them.
*/
const columns = "id, token_hash, user_id, data, flash, ip, user_agent, authenticated_at, confirmed_at, " +
	"last_activity_at, expires_at, created_at, auth_method, auth_methods, confirmed_method, device_id, " +
	"device_label, country, updated_at, sso_connection_id, auth_domain"

/*
PostgresStore keeps sessions in a PostgreSQL table.
*/
type PostgresStore struct {
	db    *database.DB
	table string
}

var _ Store = (*PostgresStore)(nil)

/*
NewPostgresStore returns a store on the table, or on DefaultTable when
the name is empty. A schema-qualified name such as "auth.sessions" is
accepted.
*/
func NewPostgresStore(db *database.DB, table string) *PostgresStore {
	if table == "" {
		table = DefaultTable
	}

	return &PostgresStore{db: db, table: pgx.Identifier(strings.Split(table, ".")).Sanitize()}
}

/*
Find returns the session whose token has the digest.
*/
func (s *PostgresStore) Find(ctx context.Context, tokenHash []byte) (Record, error) {
	sql := "SELECT " + columns + " FROM " + s.table + " WHERE token_hash = $1"

	record, err := scanRecord(s.db.QueryRow(ctx, sql, tokenHash))
	if errors.Is(err, database.ErrNotFound) {
		return Record{}, ErrNotFound
	}

	if err != nil {
		return Record{}, fmt.Errorf("find session: %w", err)
	}

	return record, nil
}

/*
Save inserts the session or replaces the stored one with the same id.
Empty sign-in values are stored as NULL, so a constraint on the table
can tell a visitor from a signed-in user.
*/
func (s *PostgresStore) Save(ctx context.Context, r Record) error {
	data, err := json.Marshal(nonNil(r.Data))
	if err != nil {
		return fmt.Errorf("encode session data: %w", err)
	}

	flash, err := json.Marshal(nonNil(r.Flash))
	if err != nil {
		return fmt.Errorf("encode session flash: %w", err)
	}

	sql := "INSERT INTO " + s.table + " (" + columns + ") " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21) " +
		"ON CONFLICT (id) DO UPDATE SET " +
		"token_hash = EXCLUDED.token_hash, user_id = EXCLUDED.user_id, data = EXCLUDED.data, " +
		"flash = EXCLUDED.flash, authenticated_at = EXCLUDED.authenticated_at, " +
		"confirmed_at = EXCLUDED.confirmed_at, last_activity_at = EXCLUDED.last_activity_at, " +
		"auth_method = EXCLUDED.auth_method, auth_methods = EXCLUDED.auth_methods, " +
		"confirmed_method = EXCLUDED.confirmed_method, device_id = EXCLUDED.device_id, " +
		"device_label = EXCLUDED.device_label, country = EXCLUDED.country, updated_at = EXCLUDED.updated_at, " +
		"sso_connection_id = EXCLUDED.sso_connection_id, auth_domain = EXCLUDED.auth_domain"

	_, err = s.db.Exec(ctx, sql, r.ID, r.TokenHash, nullID(r.UserID), data, flash, r.IP, r.UserAgent,
		nullTime(r.AuthenticatedAt), nullTime(r.ConfirmedAt), r.LastActivityAt, r.ExpiresAt, r.CreatedAt,
		nullString(r.AuthMethod), nonNilSlice(r.AuthMethods), nullString(r.ConfirmedMethod),
		nullString(r.DeviceID), nullString(r.DeviceLabel), nullString(r.Country), r.UpdatedAt,
		nullID(r.ConnectionID), nullString(r.AuthDomain))
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

/*
Touch records activity on the session.
*/
func (s *PostgresStore) Touch(ctx context.Context, id ids.UUID, at time.Time) error {
	if _, err := s.db.Exec(ctx, "UPDATE "+s.table+" SET last_activity_at = $2 WHERE id = $1", id, at); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}

	return nil
}

/*
Delete removes the session.
*/
func (s *PostgresStore) Delete(ctx context.Context, id ids.UUID) error {
	if _, err := s.db.Exec(ctx, "DELETE FROM "+s.table+" WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

/*
ForUser returns the user's sessions, most recently active first.
*/
func (s *PostgresStore) ForUser(ctx context.Context, userID ids.UUID) ([]Record, error) {
	sql := "SELECT " + columns + " FROM " + s.table + " WHERE user_id = $1 ORDER BY last_activity_at DESC, id"

	rows, err := s.db.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var records []Record

	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("read session: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	return records, nil
}

/*
DeleteForUser removes the user's sessions except the one kept.
*/
func (s *PostgresStore) DeleteForUser(ctx context.Context, userID, keep ids.UUID) (int64, error) {
	tag, err := s.db.Exec(ctx, "DELETE FROM "+s.table+" WHERE user_id = $1 AND id <> $2", userID, keep)
	if err != nil {
		return 0, fmt.Errorf("delete sessions of user: %w", err)
	}

	return tag.RowsAffected(), nil
}

/*
Prune deletes sessions past their absolute expiry, or idle since before
the cutoff, and returns how many it deleted. A scheduled job runs it so
abandoned sessions do not pile up.
*/
func (s *PostgresStore) Prune(ctx context.Context, now, idleCutoff time.Time) (int64, error) {
	sql := "DELETE FROM " + s.table + " WHERE expires_at <= $1 OR last_activity_at < $2"

	tag, err := s.db.Exec(ctx, sql, now, idleCutoff)
	if err != nil {
		return 0, fmt.Errorf("prune sessions: %w", err)
	}

	return tag.RowsAffected(), nil
}

func scanRecord(row pgx.Row) (Record, error) {
	var (
		r                          Record
		userID                     *ids.UUID
		data, flash                []byte
		ip, userAgent              *string
		authenticatedAt            *time.Time
		confirmedAt                *time.Time
		authMethod, confirmed      *string
		deviceID, deviceLabel, cty *string
		connectionID               *ids.UUID
		authDomain                 *string
	)

	err := row.Scan(&r.ID, &r.TokenHash, &userID, &data, &flash, &ip, &userAgent, &authenticatedAt,
		&confirmedAt, &r.LastActivityAt, &r.ExpiresAt, &r.CreatedAt, &authMethod, &r.AuthMethods, &confirmed,
		&deviceID, &deviceLabel, &cty, &r.UpdatedAt, &connectionID, &authDomain)
	if err != nil {
		return Record{}, err
	}

	if err := json.Unmarshal(data, &r.Data); err != nil {
		return Record{}, fmt.Errorf("decode session data: %w", err)
	}

	if err := json.Unmarshal(flash, &r.Flash); err != nil {
		return Record{}, fmt.Errorf("decode session flash: %w", err)
	}

	r.UserID = deref(userID, ids.Nil())
	r.IP, r.UserAgent = deref(ip, ""), deref(userAgent, "")
	r.AuthenticatedAt, r.ConfirmedAt = deref(authenticatedAt, time.Time{}), deref(confirmedAt, time.Time{})
	r.AuthMethod, r.ConfirmedMethod = deref(authMethod, ""), deref(confirmed, "")
	r.DeviceID, r.DeviceLabel, r.Country = deref(deviceID, ""), deref(deviceLabel, ""), deref(cty, "")
	r.ConnectionID, r.AuthDomain = deref(connectionID, ids.Nil()), deref(authDomain, "")

	return r, nil
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}

/*
nonNilSlice returns an empty list for nil, which the driver would
otherwise send as NULL.
*/
func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func nullID(id ids.UUID) *ids.UUID {
	if id == ids.Nil() {
		return nil
	}

	return &id
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}

	return *p
}
