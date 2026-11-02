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
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/queue"
)

func TestWorkers(t *testing.T) {
	t.Parallel()

	t.Run("hands the worker the job", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		var got *queue.Job[mailArgs]
		register(t, q, func(_ context.Context, job *queue.Job[mailArgs]) error {
			got = job

			return nil
		})
		require.NoError(t, q.Push(t.Context(), mailArgs{To: "a@example.com"}))

		require.NoError(t, q.Drain(t.Context()))

		require.NotNil(t, got)
		assert.Positive(t, got.ID)
		assert.Equal(t, mailArgs{To: "a@example.com"}, got.Args)
		assert.Equal(t, []any{"mail", 1, 3}, []any{got.Queue, got.Attempt, got.MaxAttempts})
		assert.WithinDuration(t, clock.System().Now(), got.CreatedAt, time.Minute)
		assert.Equal(t, "completed", jobRows(t, db)[0].State)
	})

	t.Run("retries a failed job later", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return errors.New("unavailable")
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))

		require.NoError(t, q.Drain(t.Context()))

		job := jobRows(t, db)[0]
		assert.Equal(t, []any{1, 1}, []any{job.Attempt, job.Errors})
		assert.True(t, job.ScheduledAt.After(clock.System().Now()), "the retry waits")
	})

	t.Run("retries at the time the worker asks for", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return queue.RetryAfter(time.Hour)
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))

		require.NoError(t, q.Drain(t.Context()))

		job := jobRows(t, db)[0]
		assert.Equal(t, []any{"retryable", 1, 1}, []any{job.State, job.Attempt, job.Errors})
		assert.WithinDuration(t, clock.System().Now().Add(time.Hour), job.ScheduledAt, time.Minute)
	})

	t.Run("gives up a job with no attempts left", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return queue.RetryAfter(time.Hour)
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, queue.MaxAttempts(1)))

		require.NoError(t, q.Drain(t.Context()))

		assert.Equal(t, "discarded", jobRows(t, db)[0].State)
	})

	t.Run("postpones without using up an attempt", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return queue.Postpone(time.Hour)
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}, queue.MaxAttempts(1)))

		require.NoError(t, q.Drain(t.Context()))

		job := jobRows(t, db)[0]
		assert.Equal(t, []any{"scheduled", 0}, []any{job.State, job.Errors})
		assert.WithinDuration(t, clock.System().Now().Add(time.Hour), job.ScheduledAt, time.Minute)
	})

	t.Run("cancels a job for good", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return queue.Cancel(errors.New("gone"))
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))

		require.NoError(t, q.Drain(t.Context()))

		job := jobRows(t, db)[0]
		assert.Equal(t, []any{string(queue.StateCanceled), 1}, []any{job.State, job.Errors})
	})

	t.Run("refuses a second worker for a kind", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		require.NoError(t, queue.Register(q, workerFunc[greetArgs](nil)))

		err := queue.Register(q, workerFunc[greetArgs](nil))

		require.ErrorContains(t, err, "register greet")
	})
}

func TestDrain(t *testing.T) {
	t.Parallel()

	t.Run("works the jobs that drained jobs push", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(ctx context.Context, job *queue.Job[greetArgs]) error {
			return q.Push(ctx, mailArgs{To: job.Args.Name + "@example.com"})
		})
		register(t, q, func(context.Context, *queue.Job[mailArgs]) error {
			return nil
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "ada"}))

		require.NoError(t, q.Drain(t.Context()))

		jobs := jobRows(t, db)
		require.Len(t, jobs, 2)
		assert.Equal(t, []string{"completed", "completed"}, []string{jobs[0].State, jobs[1].State})
	})

	t.Run("waits for a job stamped ahead of the database clock", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return nil
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "ada"}))
		_, err := db.Exec(t.Context(), "UPDATE river_job SET scheduled_at = now() + interval '300 milliseconds'")
		require.NoError(t, err)

		require.NoError(t, q.Drain(t.Context()))

		assert.Equal(t, "completed", jobRows(t, db)[0].State)
	})

	t.Run("leaves running workers running", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			return nil
		})
		require.NoError(t, q.Start(t.Context()))

		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))
		require.NoError(t, q.Drain(t.Context()))
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Grace"}))
		require.NoError(t, q.Drain(t.Context()))

		assert.Len(t, jobRows(t, db), 2)
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		release := make(chan struct{})
		register(t, q, func(context.Context, *queue.Job[greetArgs]) error {
			<-release

			return nil
		})
		require.NoError(t, q.Push(t.Context(), greetArgs{Name: "Ada"}))
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		err := q.Drain(ctx)
		close(release)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "drain queue")
	})

	t.Run("fails without workers", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)

		require.ErrorContains(t, q.Drain(t.Context()), "start queue")
	})
}

func TestPeriodic(t *testing.T) {
	t.Parallel()

	t.Run("the leader pushes the job on start", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		done := make(chan string, 1)
		register(t, q, func(_ context.Context, job *queue.Job[greetArgs]) error {
			done <- job.Queue

			return nil
		})
		q.AddPeriodic(queue.Periodic{
			Next:       func(after time.Time) time.Time { return after.Add(time.Hour) },
			Args:       greetArgs{Name: "Ada"},
			Opts:       []queue.Opt{queue.OnQueue("mail")},
			RunOnStart: true,
		})

		require.NoError(t, q.Start(t.Context()))

		select {
		case got := <-done:
			assert.Equal(t, "mail", got)
		case <-clock.System().After(30 * time.Second):
			t.Fatal("the periodic job never ran")
		}
	})
}

func TestMigration(t *testing.T) {
	t.Parallel()

	t.Run("creates the queue tables once", func(t *testing.T) {
		t.Parallel()
		url := migratedDatabase(t)

		_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
		require.NoError(t, err)

		db := openDB(t, database.Config{URL: url, PoolMode: database.PoolModeSession, MaxConns: 1})
		var tables int
		require.NoError(t, db.QueryRow(t.Context(),
			"SELECT count(*) FROM pg_tables WHERE tablename IN ('river_job', 'river_migration')").Scan(&tables))
		assert.Equal(t, 2, tables)
	})
}
