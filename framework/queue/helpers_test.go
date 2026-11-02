package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

type greetArgs struct {
	Name string `json:"name"`
}

func (greetArgs) Kind() string { return "greet" }

type mailArgs struct {
	To string `json:"to"`
}

func (mailArgs) Kind() string { return "mail" }

func (mailArgs) Options() queue.Options {
	return queue.Options{Queue: "mail", Priority: 2, MaxAttempts: 3}
}

/*
workerFunc turns a function into a worker.
*/
type workerFunc[A queue.Args] func(ctx context.Context, job *queue.Job[A]) error

func (f workerFunc[A]) Handle(ctx context.Context, job *queue.Job[A]) error {
	return f(ctx, job)
}

/*
register registers the function as the worker for its kind.
*/
func register[A queue.Args](t *testing.T, q *queue.Queue, fn func(ctx context.Context, job *queue.Job[A]) error) {
	t.Helper()

	require.NoError(t, queue.Register(q, workerFunc[A](fn)))
}

/*
testConfig polls quickly so tests never wait on the default interval.
*/
func testConfig() queue.Config {
	return queue.Config{
		Queues:       map[string]int{"default": 4, "mail": 2},
		MaxConns:     3,
		PollInterval: 20 * time.Millisecond,
	}
}

/*
migratedDatabase creates a test database with the queue's tables.
*/
func migratedDatabase(t *testing.T) string {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
	require.NoError(t, err)

	return url
}

/*
newQueue builds a queue on the database and closes it when the test
ends, stopping its workers first.
*/
func newQueue(t *testing.T, db database.Config, cfg queue.Config) *queue.Queue {
	t.Helper()

	q, err := queue.New(t.Context(), db, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = q.Stop(context.WithoutCancel(t.Context()))
		q.Close()
	})

	return q
}

/*
setup returns a queue and an application pool on a fresh, migrated
database. Pools stay small, since many tests run at once against one
server.
*/
func setup(t *testing.T) (*queue.Queue, *database.DB) {
	t.Helper()

	cfg := database.Config{URL: migratedDatabase(t), PoolMode: database.PoolModeSession, MaxConns: 2}

	return newQueue(t, cfg, testConfig()), openDB(t, cfg)
}

func openDB(t *testing.T, cfg database.Config) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
jobRow is what a test checks about a stored job.
*/
type jobRow struct {
	Kind        string
	Queue       string
	State       string
	Args        string
	Priority    int
	MaxAttempts int
	Attempt     int
	Errors      int
	ScheduledAt time.Time
}

/*
jobRows reads every stored job, oldest first.
*/
func jobRows(t *testing.T, db *database.DB) []jobRow {
	t.Helper()

	rows, err := db.Query(t.Context(), `SELECT kind, queue, state, args::text, priority, max_attempts, attempt,
		coalesce(array_length(errors, 1), 0), scheduled_at FROM river_job ORDER BY id`)
	require.NoError(t, err)

	jobs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (jobRow, error) {
		var j jobRow
		err := row.Scan(&j.Kind, &j.Queue, &j.State, &j.Args, &j.Priority, &j.MaxAttempts, &j.Attempt,
			&j.Errors, &j.ScheduledAt)

		return j, err
	})
	require.NoError(t, err)

	return jobs
}
