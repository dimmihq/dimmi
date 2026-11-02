package events_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

type named interface {
	Subject() string
}

type userRegistered struct {
	Email string
}

func (e userRegistered) Subject() string { return e.Email }

type userTyping struct {
	events.Ephemeral

	Email string
}

type welcomeArgs struct {
	Email string `json:"email"`
}

func (welcomeArgs) Kind() string { return "welcome" }

type welcomeWorker struct{}

func (welcomeWorker) Handle(context.Context, *queue.Job[welcomeArgs]) error { return nil }

/*
recorder remembers what the listeners saw, in order.
*/
type recorder struct {
	mu   sync.Mutex
	seen []string
}

func (r *recorder) add(entry string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen = append(r.seen, entry)
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.seen...)
}

/*
fixture is a dispatcher with its queue and pool on a fresh, migrated
database. The queue only pushes, so one connection is enough.
*/
type fixture struct {
	db *database.DB
	d  *events.Dispatcher
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
	require.NoError(t, err)

	cfg := database.Config{URL: url, PoolMode: database.PoolModeSession, MaxConns: 2}
	q, err := queue.New(t.Context(), cfg, queue.Config{MaxConns: 1})
	require.NoError(t, err)
	t.Cleanup(q.Close)
	require.NoError(t, queue.Register(q, welcomeWorker{}))

	db, err := database.Open(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	_, err = db.Exec(t.Context(), "CREATE TABLE audit (entry text)")
	require.NoError(t, err)

	return fixture{db: db, d: events.New(q)}
}

func (f fixture) count(t *testing.T, sql string) int {
	t.Helper()

	var n int
	require.NoError(t, f.db.QueryRow(t.Context(), sql).Scan(&n))

	return n
}

/*
listenAll registers one listener per phase that records the event and
writes an audit row inside the transaction.
*/
func (f fixture) listenAll(r *recorder) {
	events.Listen(f.d, events.InTx, func(ctx context.Context, e userRegistered) error {
		r.add("in tx " + e.Email)
		_, err := f.db.Exec(ctx, "INSERT INTO audit VALUES ($1)", e.Email)

		return err
	})
	events.Listen(f.d, events.AfterCommit, func(_ context.Context, e userRegistered) error {
		r.add("after commit " + e.Email)

		return nil
	})
	events.ListenQueued(f.d, func(e userRegistered) queue.Args {
		r.add("queued " + e.Email)

		return welcomeArgs(e)
	})
}

func TestDispatch(t *testing.T) {
	t.Parallel()

	t.Run("runs every phase with the committed transaction", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		f.listenAll(r)

		err := f.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, f.d.Dispatch(ctx, tx, userRegistered{Email: "ada@example.com"}))
			assert.Equal(t, []string{"in tx ada@example.com", "queued ada@example.com"}, r.all())

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, []string{
			"in tx ada@example.com", "queued ada@example.com", "after commit ada@example.com",
		}, r.all())
		assert.Equal(t, 1, f.count(t, "SELECT count(*) FROM audit"))
		assert.Equal(t, 1, f.count(t, "SELECT count(*) FROM river_job WHERE kind = 'welcome'"))
	})

	t.Run("leaves no trace when the transaction rolls back", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		f.listenAll(r)
		failed := errors.New("failed")

		err := f.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, f.d.Dispatch(ctx, tx, userRegistered{Email: "ada@example.com"}))

			return failed
		})

		require.ErrorIs(t, err, failed)
		assert.NotContains(t, r.all(), "after commit ada@example.com")
		assert.Equal(t, 0, f.count(t, "SELECT count(*) FROM audit"))
		assert.Equal(t, 0, f.count(t, "SELECT count(*) FROM river_job"))
	})

	t.Run("stops at a failing in-transaction listener", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		refused := errors.New("refused")
		events.Listen(f.d, events.InTx, func(context.Context, userRegistered) error { return refused })
		f.listenAll(r)

		err := f.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return f.d.Dispatch(ctx, tx, userRegistered{Email: "ada@example.com"})
		})

		require.ErrorIs(t, err, refused)
		require.ErrorContains(t, err, "dispatch events_test.userRegistered")
		assert.Empty(t, r.all())
		assert.Equal(t, 0, f.count(t, "SELECT count(*) FROM river_job"))
	})

	t.Run("runs every listener at once without a transaction", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		f.listenAll(r)

		require.NoError(t, f.d.Dispatch(t.Context(), nil, userRegistered{Email: "ada@example.com"}))

		assert.Equal(t, []string{
			"in tx ada@example.com", "queued ada@example.com", "after commit ada@example.com",
		}, r.all())
		assert.Equal(t, 1, f.count(t, "SELECT count(*) FROM audit"))
		assert.Equal(t, 1, f.count(t, "SELECT count(*) FROM river_job"))
	})

	t.Run("logs a failing after-commit listener and runs the rest", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		events.Listen(f.d, events.AfterCommit, func(context.Context, userRegistered) error {
			return errors.New("broadcast down")
		})
		f.listenAll(r)
		var out bytes.Buffer
		logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: &out})
		require.NoError(t, err)

		ctx := log.NewContext(t.Context(), logger)

		require.NoError(t, f.d.Dispatch(ctx, nil, userRegistered{Email: "ada@example.com"}))

		assert.Contains(t, r.all(), "after commit ada@example.com")
		assert.Contains(t, out.String(), `"msg":"after-commit listener failed"`)
		assert.Contains(t, out.String(), `"event":"events_test.userRegistered"`)
		assert.Contains(t, out.String(), "broadcast down")
	})

	t.Run("reports a job that cannot be pushed", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		events.ListenQueued(f.d, func(userRegistered) queue.Args { return unknownArgs{} })

		err := f.d.Dispatch(t.Context(), nil, userRegistered{Email: "ada@example.com"})

		require.ErrorIs(t, err, queue.ErrUnknownKind)
	})

	t.Run("pushes nothing when a queued listener returns no job", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		events.ListenQueued(f.d, func(userRegistered) queue.Args { return nil })

		require.NoError(t, f.d.Dispatch(t.Context(), nil, userRegistered{Email: "ada@example.com"}))

		assert.Equal(t, 0, f.count(t, "SELECT count(*) FROM river_job"))
	})

	t.Run("hands each listener only the events it asks for", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		events.Listen(f.d, events.AfterCommit, func(_ context.Context, e named) error {
			r.add("named " + e.Subject())

			return nil
		})
		events.Listen(f.d, events.AfterCommit, func(_ context.Context, e userTyping) error {
			r.add("typing " + e.Email)

			return nil
		})

		require.NoError(t, f.d.Dispatch(t.Context(), nil, userRegistered{Email: "ada@example.com"}))
		require.NoError(t, f.d.Dispatch(t.Context(), nil, userTyping{Email: "grace@example.com"}))

		assert.Equal(t, []string{"named ada@example.com", "typing grace@example.com"}, r.all())
	})

	t.Run("hands ephemeral events to after-commit listeners only", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		r := &recorder{}
		events.Listen(f.d, events.InTx, func(context.Context, userTyping) error {
			r.add("in tx")

			return nil
		})
		events.ListenQueued(f.d, func(userTyping) queue.Args {
			r.add("queued")

			return welcomeArgs{}
		})
		events.Listen(f.d, events.AfterCommit, func(context.Context, userTyping) error {
			r.add("after commit")

			return nil
		})

		err := f.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return f.d.Dispatch(ctx, tx, userTyping{Email: "ada@example.com"})
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"after commit"}, r.all())
		assert.Equal(t, 0, f.count(t, "SELECT count(*) FROM river_job"))
	})
}

func TestIsEphemeral(t *testing.T) {
	t.Parallel()

	t.Run("tells ephemeral events from stored ones", func(t *testing.T) {
		t.Parallel()

		assert.True(t, events.IsEphemeral(userTyping{}))
		assert.True(t, events.IsEphemeral(&userTyping{}))
		assert.False(t, events.IsEphemeral(userRegistered{}))
	})
}

type unknownArgs struct{}

func (unknownArgs) Kind() string { return "unknown" }
