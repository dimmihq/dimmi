package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("refuses a transaction pooler without a direct url", func(t *testing.T) {
		t.Parallel()

		db := database.Config{URL: "postgres://pooler/app", PoolMode: database.PoolModeTransaction}

		_, err := queue.New(t.Context(), db, queue.Config{})

		require.ErrorIs(t, err, database.ErrNoSessionURL)
	})

	t.Run("rejects a malformed url", func(t *testing.T) {
		t.Parallel()

		_, err := queue.New(t.Context(), database.Config{URL: "postgres://%zz"}, queue.Config{})

		require.ErrorContains(t, err, "parse queue url")
	})

	t.Run("rejects an invalid queue", func(t *testing.T) {
		t.Parallel()

		_, err := queue.New(t.Context(), database.Config{URL: "postgres://localhost/app"}, queue.Config{
			Queues: map[string]int{"default": -1},
		})

		require.ErrorContains(t, err, "create queue client")
	})
}

func TestPush(t *testing.T) {
	t.Parallel()

	t.Run("stores the job with the kind's options and the push's overrides", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[mailArgs](nil)))
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		require.NoError(t, q.Push(t.Context(), mailArgs{To: "a@example.com"}))
		require.NoError(t, q.Push(t.Context(), mailArgs{To: "b@example.com"}, queue.OnQueue("default"),
			queue.Priority(4), queue.MaxAttempts(9)))
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))

		jobs := jobRows(t, db)
		require.Len(t, jobs, 3)
		assert.Equal(t, jobRow{
			Kind: "mail", Queue: "mail", State: "available", Args: `{"to": "a@example.com"}`,
			Priority: 2, MaxAttempts: 3, ScheduledAt: jobs[0].ScheduledAt,
		}, jobs[0])
		assert.Equal(t, []any{"default", 4, 9}, []any{jobs[1].Queue, jobs[1].Priority, jobs[1].MaxAttempts})
		assert.Equal(t, []any{"default", 1, 25}, []any{jobs[2].Queue, jobs[2].Priority, jobs[2].MaxAttempts})
	})

	t.Run("holds a job until its time", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))
		at := clock.System().Now().Add(time.Hour)

		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, queue.At(at)))

		jobs := jobRows(t, db)
		require.Len(t, jobs, 1)
		assert.Equal(t, "scheduled", jobs[0].State)
		assert.WithinDuration(t, at, jobs[0].ScheduledAt, time.Millisecond)
	})

	t.Run("holds a job for its time on the queue's clock, however far that clock is from now", func(t *testing.T) {
		t.Parallel()
		appClock := clock.NewFake(time.Date(2020, 1, 1, 9, 0, 0, 0, time.UTC))
		cfg := testConfig()
		cfg.Clock = appClock
		dbCfg := database.Config{URL: migratedDatabase(t), PoolMode: database.PoolModeSession, MaxConns: 2}
		q, db := newQueue(t, dbCfg, cfg), openDB(t, dbCfg)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, queue.At(appClock.Now().Add(time.Hour))))

		jobs := jobRows(t, db)
		require.Len(t, jobs, 1)
		assert.Equal(t, "scheduled", jobs[0].State)
		assert.WithinDuration(t, clock.System().Now().Add(time.Hour), jobs[0].ScheduledAt, time.Minute)
	})

	t.Run("skips a unique job that already waits", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))
		unique := queue.Unique(queue.UniqueOpts{ByArgs: true, ByPeriod: time.Hour, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRunning, queue.StateScheduled, queue.StateRetryable,
		}})

		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, unique))
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, unique))
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Grace"}, unique))

		assert.Len(t, jobRows(t, db), 2)
	})

	t.Run("refuses a kind without a worker", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)

		err := q.Push(t.Context(), greetArgs{Name: "Ada"})

		require.ErrorIs(t, err, queue.ErrUnknownKind)
		require.ErrorContains(t, err, "push greet")
		assert.Empty(t, jobRows(t, db))
	})

	t.Run("reports a failed insert", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		err := q.Push(t.Context(), greetArgs{Name: "Ada"}, queue.Priority(9))

		require.ErrorContains(t, err, "push greet")
	})
}

func TestPushTx(t *testing.T) {
	t.Parallel()

	t.Run("keeps the job when the transaction commits", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return q.PushTx(ctx, tx, greetArgs{Name: "Ada"})
		})

		require.NoError(t, err)
		jobs := jobRows(t, db)
		require.Len(t, jobs, 1)
		assert.JSONEq(t, `{"name":"Ada"}`, jobs[0].Args)
	})

	t.Run("drops the job when the transaction rolls back", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))
		failed := errors.New("failed")

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, q.PushTx(ctx, tx, greetArgs{Name: "Ada"}))

			return failed
		})

		require.ErrorIs(t, err, failed)
		assert.Empty(t, jobRows(t, db))
	})

	t.Run("pushes through a transaction pooler's statement mode", func(t *testing.T) {
		t.Parallel()
		url := migratedDatabase(t)
		cfg := database.Config{URL: url, DirectURL: url, PoolMode: database.PoolModeTransaction, MaxConns: 1}
		q := newQueue(t, cfg, testConfig())
		var got greetArgs
		register(t, q, func(_ context.Context, job *queue.Job[greetArgs]) error {
			got = job.Args

			return nil
		})

		err := openDB(t, cfg).Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return q.PushTx(ctx, tx, greetArgs{Name: "Ada"})
		})
		require.NoError(t, err)

		require.NoError(t, q.Drain(t.Context()))
		assert.Equal(t, greetArgs{Name: "Ada"}, got)
	})

	t.Run("reports a failed insert", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			_, _ = tx.Exec(ctx, "SELECT missing_column FROM river_job")

			return q.PushTx(ctx, tx, greetArgs{Name: "Ada"})
		})

		require.ErrorContains(t, err, "push greet")
	})
}

func TestLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("works jobs while started", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		done := make(chan string, 1)
		register(t, q, func(_ context.Context, job *queue.Job[greetArgs]) error {
			done <- job.Args.Name

			return nil
		})
		require.NoError(t, q.Start(t.Context()))

		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))

		assert.Equal(t, "Ada", <-done)
		require.NoError(t, q.Stop(t.Context()))
		assert.Equal(t, "completed", jobRows(t, db)[0].State)
	})

	t.Run("refuses to start without workers", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)

		require.ErrorContains(t, q.Start(t.Context()), "start queue")
	})

	t.Run("cancels running jobs when the stop deadline passes", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		started := make(chan struct{})
		canceled := make(chan struct{})
		register(t, q, func(ctx context.Context, _ *queue.Job[greetArgs]) error {
			close(started)
			<-ctx.Done()
			close(canceled)

			return ctx.Err()
		})
		require.NoError(t, q.Start(t.Context()))
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))
		<-started
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		err := q.Stop(ctx)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		<-canceled
	})
}

func TestNoLeader(t *testing.T) {
	t.Parallel()

	t.Run("registers no periodic jobs on a node outside leader election", func(t *testing.T) {
		t.Parallel()
		cfg := testConfig()
		cfg.NoLeader = true
		db := database.Config{URL: migratedDatabase(t), PoolMode: database.PoolModeSession, MaxConns: 2}
		q := newQueue(t, db, cfg)

		assert.NotPanics(t, func() {
			q.AddPeriodic(queue.Periodic{
				Next: func(after time.Time) time.Time { return after.Add(time.Hour) },
				Args: greetArgs{Name: "ana"},
			})
		})
	})
}
