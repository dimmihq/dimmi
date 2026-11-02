/*
|--------------------------------------------------------------------------
| Certificate Storage
|--------------------------------------------------------------------------
|
| Certificates must survive restarts and be shared by every server:
| obtaining one again on each boot would quickly hit the certificate
| authority's rate limits, and a server that cannot see its neighbour's
| certificate would fail handshakes for that hostname. PostgreSQL is
| already durable and shared, so certificates live there, in one table
| of keys and values.
|
| Keys are slash-separated paths, like files in directories. A key that
| prefixes others is a directory: it exists, can be listed, and deleting
| it deletes everything under it.
|
*/

package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
)

/*
Storage is a certmagic storage and locker on PostgreSQL.
*/
type Storage struct {
	db    *database.DB
	clock clock.Clock

	mu   sync.Mutex
	held map[string]*heldLock
}

var _ certmagic.Storage = (*Storage)(nil)

/*
New returns storage on the database, which must have the table from
Migrations. The clock stamps each write.
*/
func New(db *database.DB, clk clock.Clock) *Storage {
	return &Storage{db: db, clock: clk, held: map[string]*heldLock{}}
}

/*
Store writes the value at the key, replacing any value already there.
*/
func (s *Storage) Store(ctx context.Context, key string, value []byte) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO certmagic_storage (key, value, modified) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, modified = excluded.modified`,
		clean(key), value, s.clock.Now())
	if err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}

	return nil
}

/*
Load returns the value at the key, or an error wrapping fs.ErrNotExist
when there is none.
*/
func (s *Storage) Load(ctx context.Context, key string) ([]byte, error) {
	var value []byte

	err := s.db.QueryRow(ctx, "SELECT value FROM certmagic_storage WHERE key = $1", clean(key)).Scan(&value)
	if errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("load %s: %w", key, fs.ErrNotExist)
	}

	if err != nil {
		return nil, fmt.Errorf("load %s: %w", key, err)
	}

	return value, nil
}

/*
Delete removes the key and every key under it. A key that does not
exist is already deleted.
*/
func (s *Storage) Delete(ctx context.Context, key string) error {
	key = clean(key)

	_, err := s.db.Exec(ctx,
		"DELETE FROM certmagic_storage WHERE key = $1 OR starts_with(key, $2)", key, key+"/")
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}

	return nil
}

/*
Exists reports whether the key holds a value or prefixes keys that do.
A failed query reports false.
*/
func (s *Storage) Exists(ctx context.Context, key string) bool {
	key = clean(key)

	var exists bool

	err := s.db.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM certmagic_storage WHERE key = $1 OR starts_with(key, $2))",
		key, key+"/").Scan(&exists)

	return err == nil && exists
}

/*
List returns the keys under the directory. Without recursion it returns
the directory's direct entries, files and subdirectories alike; with it,
every entry at any depth. An empty directory does not exist and returns
an error wrapping fs.ErrNotExist.
*/
func (s *Storage) List(ctx context.Context, dir string, recursive bool) ([]string, error) {
	dir = clean(dir)

	prefix := dir + "/"
	if dir == "" {
		prefix = ""
	}

	rows, err := s.db.Query(ctx, "SELECT key FROM certmagic_storage WHERE starts_with(key, $1)", prefix)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	defer rows.Close()

	var entries []string

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("list %s: %w", dir, err)
		}

		entries = append(entries, pathsTo(dir, strings.Split(strings.TrimPrefix(key, prefix), "/"))...)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("list %s: %w", dir, fs.ErrNotExist)
	}

	entries = slices.DeleteFunc(entries, func(entry string) bool {
		return !recursive && strings.Contains(strings.TrimPrefix(entry, prefix), "/")
	})
	slices.Sort(entries)

	return slices.Compact(entries), nil
}

/*
Stat describes the key: a terminal key with its size and modification
time, or a directory. A key that is neither returns an error wrapping
fs.ErrNotExist.
*/
func (s *Storage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	key = clean(key)

	var (
		size     int64
		modified time.Time
	)

	err := s.db.QueryRow(ctx,
		"SELECT octet_length(value), modified FROM certmagic_storage WHERE key = $1", key).Scan(&size, &modified)
	if err == nil {
		return certmagic.KeyInfo{Key: key, Modified: modified, Size: size, IsTerminal: true}, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return certmagic.KeyInfo{}, fmt.Errorf("stat %s: %w", key, err)
	}

	if !s.Exists(ctx, key) {
		return certmagic.KeyInfo{}, fmt.Errorf("stat %s: %w", key, fs.ErrNotExist)
	}

	return certmagic.KeyInfo{Key: key}, nil
}

/*
pathsTo returns every path from the directory down the parts: "a/b"
with parts c and d gives "a/b/c" and "a/b/c/d".
*/
func pathsTo(dir string, parts []string) []string {
	paths := make([]string, len(parts))
	for i := range parts {
		paths[i] = path.Join(dir, path.Join(parts[:i+1]...))
	}

	return paths
}

func clean(key string) string {
	return strings.Trim(key, "/")
}
