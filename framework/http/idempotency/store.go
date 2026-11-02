/*
|--------------------------------------------------------------------------
| Idempotency Store
|--------------------------------------------------------------------------
|
| A client that loses the answer to a write cannot tell whether the write
| happened, so it sends the same key again. The store reserves a key for
| the first request, keeps its response once it finishes, and hands that
| response back to every retry until the key expires. A retry with the
| same key but a different request is a client bug and is refused rather
| than answered with a response that belongs to something else.
|
| A reservation that is never completed, because the work failed or the
| process died, does not hold the key forever: the caller releases it on
| failure, and a reservation older than the lock timeout may be taken
| over by the next attempt.
|
*/

package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
Errors a store returns from Reserve.

ErrInProgress means another request with the key has not finished yet.
ErrKeyReused means the key was first used for a different request.
*/
var (
	ErrInProgress = errors.New("idempotency: a request with this key is in progress")
	ErrKeyReused  = errors.New("idempotency: key used for a different request")
)

/*
Response is a stored response: its status, headers and body.
*/
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

/*
Store keeps responses by key. Keys are kept apart per scope, such as the
calling user, and per tenant where the store supports tenants.
*/
type Store interface {
	/*
		Reserve claims the key for a request identified by its fingerprint.
		It returns nil when the request should run, the stored response when
		it already ran, ErrInProgress or ErrKeyReused.
	*/
	Reserve(ctx context.Context, scope, key, fingerprint string) (*Response, error)

	/*
		Complete stores the response of a reserved key.
	*/
	Complete(ctx context.Context, scope, key string, res Response) error

	/*
		Release gives up a reservation that has no response, so the next
		attempt runs again.
	*/
	Release(ctx context.Context, scope, key string) error
}

/*
Config configures the database store.

Table names the table, optionally schema-qualified. TTL is how long a
response is kept. LockTimeout is how long a reservation without a
response blocks other attempts before one may take it over; keep it
longer than the slowest request.
*/
type Config struct {
	Table       string        `default:"idempotency_keys" koanf:"table"`
	TTL         time.Duration `default:"24h"              koanf:"ttl"`
	LockTimeout time.Duration `default:"5m"               koanf:"lock_timeout"`
}

/*
DatabaseStore keeps responses in a PostgreSQL table with this shape:

	CREATE TABLE idempotency_keys (
	    tenant_id   text        NOT NULL DEFAULT '',
	    scope       text        NOT NULL,
	    key         text        NOT NULL,
	    fingerprint text        NOT NULL,
	    status      integer,
	    header      jsonb,
	    body        bytea,
	    created_at  timestamptz NOT NULL,
	    expires_at  timestamptz NOT NULL,
	    PRIMARY KEY (tenant_id, scope, key)
	);

The tenant comes from the context, empty outside a tenant. Every call
runs in its own transaction so row-level security on tenant_id applies.
*/
type DatabaseStore struct {
	db    *database.DB
	table string
	cfg   Config
	clock clock.Clock
}

/*
NewDatabaseStore returns a store on the table. Zero settings fall back
to their defaults.
*/
func NewDatabaseStore(db *database.DB, cfg Config, clk clock.Clock) *DatabaseStore {
	if cfg.Table == "" {
		cfg.Table = "idempotency_keys"
	}

	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}

	if cfg.LockTimeout <= 0 {
		cfg.LockTimeout = 5 * time.Minute
	}

	table := pgx.Identifier(strings.Split(cfg.Table, ".")).Sanitize()

	return &DatabaseStore{db: db, table: table, cfg: cfg, clock: clk}
}

/*
Reserve claims the key. An expired key, or a reservation older than the
lock timeout, is taken over as if it were new.
*/
func (s *DatabaseStore) Reserve(ctx context.Context, scope, key, fingerprint string) (*Response, error) {
	id := newRef(ctx, scope, key)

	var res *Response

	err := s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		claimed, err := s.claim(ctx, tx, id, fingerprint)
		if err != nil || claimed {
			return err
		}

		res, err = s.existing(ctx, tx, id, fingerprint)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("reserve idempotency key: %w", err)
	}

	return res, nil
}

/*
Complete stores the response. Set-Cookie headers are dropped: a retry
must never receive another caller's session.
*/
func (s *DatabaseStore) Complete(ctx context.Context, scope, key string, res Response) error {
	header := res.Header.Clone()
	header.Del("Set-Cookie")

	encoded, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("encode response header: %w", err)
	}

	id := newRef(ctx, scope, key)
	sql := "UPDATE " + s.table + " SET status = $4, header = $5, body = $6 " +
		"WHERE tenant_id = $1 AND scope = $2 AND key = $3"

	return s.exec(ctx, "complete idempotency key", sql, id.tenant, id.scope, id.key, res.Status, encoded, res.Body)
}

/*
Release deletes a reservation that has no response yet.
*/
func (s *DatabaseStore) Release(ctx context.Context, scope, key string) error {
	id := newRef(ctx, scope, key)
	sql := "DELETE FROM " + s.table + " WHERE tenant_id = $1 AND scope = $2 AND key = $3 AND status IS NULL"

	return s.exec(ctx, "release idempotency key", sql, id.tenant, id.scope, id.key)
}

/*
Prune deletes expired keys of the tenant in the context and returns how
many it deleted.
*/
func (s *DatabaseStore) Prune(ctx context.Context) (int64, error) {
	var deleted int64

	err := s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM "+s.table+" WHERE expires_at <= $1", s.clock.Now())
		deleted = tag.RowsAffected()

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("prune idempotency keys: %w", err)
	}

	return deleted, nil
}

/*
claim inserts the reservation, or takes over an expired or abandoned
one, and reports whether it did.
*/
func (s *DatabaseStore) claim(ctx context.Context, tx *database.Tx, id ref, fingerprint string) (bool, error) {
	now := s.clock.Now()
	sql := "INSERT INTO " + s.table + " AS k (tenant_id, scope, key, fingerprint, created_at, expires_at) " +
		"VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (tenant_id, scope, key) DO UPDATE SET " +
		"fingerprint = EXCLUDED.fingerprint, status = NULL, header = NULL, body = NULL, " +
		"created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at " +
		"WHERE k.expires_at <= $5 OR (k.status IS NULL AND k.created_at <= $7) RETURNING true"

	var claimed bool

	err := tx.QueryRow(ctx, sql, id.tenant, id.scope, id.key, fingerprint, now, now.Add(s.cfg.TTL),
		now.Add(-s.cfg.LockTimeout)).Scan(&claimed)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	return claimed, err
}

/*
existing reads the key's current holder and decides what the new
request gets.
*/
func (s *DatabaseStore) existing(ctx context.Context, tx *database.Tx, id ref, fingerprint string) (*Response, error) {
	sql := "SELECT fingerprint, status, header, body FROM " + s.table +
		" WHERE tenant_id = $1 AND scope = $2 AND key = $3"

	var (
		stored string
		status *int
		header []byte
		body   []byte
	)

	if err := tx.QueryRow(ctx, sql, id.tenant, id.scope, id.key).Scan(&stored, &status, &header, &body); err != nil {
		return nil, err
	}

	if stored != fingerprint {
		return nil, ErrKeyReused
	}

	if status == nil {
		return nil, ErrInProgress
	}

	res := &Response{Status: *status, Body: body}
	if err := json.Unmarshal(header, &res.Header); err != nil {
		return nil, fmt.Errorf("decode response header: %w", err)
	}

	return res, nil
}

func (s *DatabaseStore) exec(ctx context.Context, action, sql string, args ...any) error {
	err := s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := tx.Exec(ctx, sql, args...)

		return err
	})
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	return nil
}

/*
ref identifies one key: the tenant from the context, the scope and the
key itself.
*/
type ref struct {
	tenant string
	scope  string
	key    string
}

func newRef(ctx context.Context, scope, key string) ref {
	tenant, _ := tenancy.From(ctx)

	return ref{tenant: tenant, scope: scope, key: key}
}
