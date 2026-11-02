package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/schedule"
	"github.com/dimmihq/dimmi/framework/testkit"
)

type pruneArgs struct {
	Table string `json:"table"`
}

func (pruneArgs) Kind() string { return "prune" }

type pruneWorker chan pruneArgs

func (w pruneWorker) Handle(_ context.Context, job *queue.Job[pruneArgs]) error {
	w <- job.Args

	return nil
}

/*
newQueue builds a queue on a fresh, migrated database and stops it when
the test ends.
*/
func newQueue(t *testing.T) *queue.Queue {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
	require.NoError(t, err)

	q, err := queue.New(t.Context(), database.Config{URL: url, PoolMode: database.PoolModeSession},
		queue.Config{MaxConns: 3, PollInterval: 20 * time.Millisecond})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = q.Stop(context.WithoutCancel(t.Context()))
		q.Close()
	})

	return q
}

func TestTask(t *testing.T) {
	t.Parallel()

	paris, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)
	now := time.Date(2026, 11, 2, 9, 30, 15, 0, time.UTC)

	tests := []struct {
		name string
		task func(task *schedule.Task) *schedule.Task
		want time.Time
	}{
		{
			name: "runs every minute by default",
			task: func(task *schedule.Task) *schedule.Task { return task },
			want: time.Date(2026, 11, 2, 9, 31, 0, 0, time.UTC),
		},
		{
			name: "runs every minute",
			task: (*schedule.Task).EveryMinute,
			want: time.Date(2026, 11, 2, 9, 31, 0, 0, time.UTC),
		},
		{
			name: "runs hourly",
			task: (*schedule.Task).Hourly,
			want: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "runs daily at midnight",
			task: (*schedule.Task).Daily,
			want: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "runs daily at a time",
			task: func(task *schedule.Task) *schedule.Task { return task.DailyAt("3:05") },
			want: time.Date(2026, 11, 3, 3, 5, 0, 0, time.UTC),
		},
		{
			name: "runs daily at a time in a timezone",
			task: func(task *schedule.Task) *schedule.Task { return task.DailyAt("17:30").Timezone(paris) },
			want: time.Date(2026, 11, 2, 16, 30, 0, 0, time.UTC),
		},
		{
			name: "runs on a cron expression",
			task: func(task *schedule.Task) *schedule.Task { return task.Cron("15 */6 * * 1-5") },
			want: time.Date(2026, 11, 2, 12, 15, 0, 0, time.UTC),
		},
		{
			name: "has no next run while the time is invalid",
			task: func(task *schedule.Task) *schedule.Task { return task.DailyAt("25:00") },
		},
		{
			name: "has no next run while the cron expression is invalid",
			task: func(task *schedule.Task) *schedule.Task { return task.Cron("every day") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			task := test.task(schedule.New(nil).Job(pruneArgs{}))

			got := task.Next(now)

			assert.True(t, test.want.Equal(got), "want %s, got %s", test.want, got)
		})
	}
}

func TestApply(t *testing.T) {
	t.Parallel()

	t.Run("reports a malformed time", func(t *testing.T) {
		t.Parallel()
		s := schedule.New(nil)
		s.Job(pruneArgs{}).Hourly()
		s.Job(pruneArgs{}).DailyAt("noon")

		err := s.Apply()

		require.ErrorIs(t, err, schedule.ErrInvalidTime)
		require.ErrorContains(t, err, `schedule prune`)
	})

	t.Run("reports a malformed cron expression", func(t *testing.T) {
		t.Parallel()
		s := schedule.New(nil)
		s.Job(pruneArgs{}).Cron("* *")

		require.ErrorContains(t, s.Apply(), `schedule prune: cron "* *"`)
	})

	t.Run("the leader pushes the tasks", func(t *testing.T) {
		t.Parallel()
		q := newQueue(t)
		worker := make(pruneWorker, 1)
		require.NoError(t, queue.Register(q, worker))
		s := schedule.New(q)
		s.Job(pruneArgs{Table: "sessions"}).DailyAt("03:00").RunOnStart()
		s.Job(pruneArgs{Table: "tokens"}).Hourly()

		require.NoError(t, s.Apply())
		require.NoError(t, q.Start(t.Context()))

		select {
		case got := <-worker:
			assert.Equal(t, pruneArgs{Table: "sessions"}, got)
		case <-clock.System().After(30 * time.Second):
			t.Fatal("the scheduled job never ran")
		}
	})
}
