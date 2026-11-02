/*
|--------------------------------------------------------------------------
| PostgreSQL Keyset Store for Tests
|--------------------------------------------------------------------------
|
| Code that encrypts needs somewhere to keep keysets, and tests use real
| PostgreSQL for that, as production does. The table here is the minimum
| the store contract needs; the application's own store lives beside its
| other tables.
|
*/

package drivertest

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
keysetsTable is the table the test store creates.
*/
const keysetsTable = `CREATE TABLE keysets (
	scope_kind text NOT NULL,
	scope_id text NOT NULL,
	purpose text NOT NULL,
	wrapped bytea NOT NULL,
	PRIMARY KEY (scope_kind, scope_id, purpose)
)`

/*
PostgresStore is a keyset store on a throwaway PostgreSQL database.
*/
type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ encryption.KeysetStore = (*PostgresStore)(nil)

/*
NewPostgresStore returns an empty store in a database of the test's own.
*/
func NewPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), testkit.NewDatabase(t).URL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	_, err = pool.Exec(t.Context(), keysetsTable)
	require.NoError(t, err)

	return &PostgresStore{pool: pool}
}

/*
Find returns the wrapped keyset.
*/
func (s *PostgresStore) Find(ctx context.Context, ref encryption.KeysetRef) ([]byte, error) {
	var wrapped []byte

	err := s.pool.QueryRow(ctx,
		`SELECT wrapped FROM keysets WHERE scope_kind = $1 AND scope_id = $2 AND purpose = $3`,
		ref.Scope.Kind, ref.Scope.ID, ref.Purpose,
	).Scan(&wrapped)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, encryption.ErrKeysetNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find keyset: %w", err)
	}

	return wrapped, nil
}

/*
Create inserts the keyset unless one exists.
*/
func (s *PostgresStore) Create(ctx context.Context, ref encryption.KeysetRef, wrapped []byte) error {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO keysets VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		ref.Scope.Kind, ref.Scope.ID, ref.Purpose, wrapped,
	)
	if err != nil {
		return fmt.Errorf("create keyset: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return encryption.ErrKeysetExists
	}

	return nil
}

/*
Replace updates the keyset only while it still holds previous.
*/
func (s *PostgresStore) Replace(ctx context.Context, ref encryption.KeysetRef, previous, next []byte) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE keysets SET wrapped = $5
		WHERE scope_kind = $1 AND scope_id = $2 AND purpose = $3 AND wrapped = $4`,
		ref.Scope.Kind, ref.Scope.ID, ref.Purpose, previous, next,
	)
	if err != nil {
		return fmt.Errorf("replace keyset: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return encryption.ErrKeysetChanged
	}

	return nil
}

/*
All yields every stored keyset's reference.
*/
func (s *PostgresStore) All(ctx context.Context) iter.Seq2[encryption.KeysetRef, error] {
	return func(yield func(encryption.KeysetRef, error) bool) {
		rows, err := s.pool.Query(ctx, `SELECT scope_kind, scope_id, purpose FROM keysets ORDER BY 1, 2, 3`)
		if err != nil {
			yield(encryption.KeysetRef{}, fmt.Errorf("list keysets: %w", err))

			return
		}

		refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (encryption.KeysetRef, error) {
			var ref encryption.KeysetRef
			err := row.Scan(&ref.Scope.Kind, &ref.Scope.ID, &ref.Purpose)

			return ref, err
		})
		if err != nil {
			yield(encryption.KeysetRef{}, fmt.Errorf("list keysets: %w", err))

			return
		}

		for _, ref := range refs {
			if !yield(ref, nil) {
				return
			}
		}
	}
}
