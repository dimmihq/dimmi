package notifications_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

type user struct {
	ID string
}

func (u user) NotifiableType() string { return "user" }

func (u user) NotifiableID() string { return u.ID }

func (u user) RouteFor(channel string) any { return channel + ":" + u.ID }

/*
greeting goes to the channels it lists.
*/
type greeting struct {
	Text     string   `json:"text"`
	Channels []string `json:"channels"`
}

func (greeting) Type() string { return "greeting" }

func (g greeting) Via(notifications.Notifiable) []string { return g.Channels }

/*
reminder decides at send time and picks its own queue options.
*/
type reminder struct {
	Text  string    `json:"text"`
	Skip  []string  `json:"skip"`
	Fail  bool      `json:"fail"`
	Queue string    `json:"queue"`
	At    time.Time `json:"at"`
}

func (reminder) Type() string { return "reminder" }

func (reminder) Via(notifications.Notifiable) []string { return []string{"feed", "pager"} }

func (r reminder) ShouldSend(_ context.Context, d notifications.Delivery) (bool, error) {
	if r.Fail {
		return false, errors.New("preferences unavailable")
	}

	return !slices.Contains(r.Skip, d.Channel), nil
}

func (r reminder) QueueOptions(notifications.Delivery) []queue.Opt {
	var opts []queue.Opt
	if r.Queue != "" {
		opts = append(opts, queue.OnQueue(r.Queue))
	}

	if !r.At.IsZero() {
		opts = append(opts, queue.At(r.At))
	}

	return opts
}

/*
unencodable cannot be stored as JSON.
*/
type unencodable struct {
	Done chan struct{}
}

func (unencodable) Type() string { return "unencodable" }

func (unencodable) Via(notifications.Notifiable) []string { return []string{"feed"} }

/*
brittle encodes but never decodes again.
*/
type brittle struct {
	Text string `json:"text"`
}

func (brittle) Type() string { return "brittle" }

func (brittle) Via(notifications.Notifiable) []string { return []string{"feed"} }

func (*brittle) UnmarshalJSON([]byte) error { return errors.New("shape changed") }

/*
sent is one delivery a recorder received.
*/
type sent struct {
	ID      string
	Channel string
	To      string
	Type    string
	Route   any
}

/*
recorder is a channel that records deliveries and fails for the
recipients it is told to.
*/
type recorder struct {
	mu    sync.Mutex
	sent  []sent
	fails map[string]error
}

func newRecorder() *recorder {
	return &recorder{fails: map[string]error{}}
}

func (r *recorder) Send(_ context.Context, d notifications.Delivery, n notifications.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.fails[d.Notifiable.NotifiableID()]; err != nil {
		return err
	}

	r.sent = append(r.sent, sent{
		ID:      d.ID,
		Channel: d.Channel,
		To:      d.Notifiable.NotifiableID(),
		Type:    n.Type(),
		Route:   d.Notifiable.RouteFor(d.Channel),
	})

	return nil
}

func (r *recorder) failFor(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.fails[id] = err
}

func (r *recorder) all() []sent {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.sent)
}

/*
channelConfig declares two channels on the recording driver.
*/
func channelConfig() notifications.Config {
	return notifications.Config{
		Queue: "default",
		Channels: map[string]config.Section{
			"feed":  {"driver": "record"},
			"pager": {"driver": "record"},
		},
	}
}

/*
newManager returns a manager on the queue whose channels all record
into the returned recorder, with the test types and users registered.
*/
func newManager(t *testing.T, q *queue.Queue, users ...user) (*notifications.Manager, *recorder) {
	t.Helper()

	m, err := notifications.New(channelConfig(), q)
	require.NoError(t, err)

	rec := newRecorder()
	m.Extend("record", func(string, config.Section) (notifications.Channel, error) { return rec, nil })
	notifications.Register[greeting](m)
	notifications.Register[reminder](m)
	notifications.Register[brittle](m)
	m.Resolve("user", func(_ context.Context, id string) (notifications.Notifiable, error) {
		for _, u := range users {
			if u.ID == id {
				return u, nil
			}
		}

		return nil, notifications.ErrNotifiableGone
	})

	return m, rec
}

/*
setup returns a queue and an application pool on a fresh database with
the queue's tables.
*/
func setup(t *testing.T) (*queue.Queue, *database.DB) {
	t.Helper()

	cfg, db := migrated(t)

	return newQueue(t, cfg), db
}

/*
migrated creates a database with the queue's tables and opens a pool on
it. Pools stay small, since many tests share one server.
*/
func migrated(t *testing.T) (database.Config, *database.DB) {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
	require.NoError(t, err)

	cfg := database.Config{URL: url, PoolMode: database.PoolModeSession, MaxConns: 2}

	db, err := database.Open(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return cfg, db
}

/*
newQueue builds a quickly polling queue on the database and closes it
when the test ends.
*/
func newQueue(t *testing.T, cfg database.Config) *queue.Queue {
	t.Helper()

	q, err := queue.New(t.Context(), cfg, queue.Config{
		Queues:       map[string]int{"default": 2, "later": 1},
		MaxConns:     2,
		PollInterval: 20 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = q.Stop(context.WithoutCancel(t.Context()))
		q.Close()
	})

	return q
}

/*
job is what a test checks about a stored job.
*/
type job struct {
	Queue string
	State string
}

/*
jobs reads every stored job, oldest first. A job that failed and will
run again is "retrying", whether its retry is due yet or not; a job
given up is "given up".
*/
func jobs(t *testing.T, db *database.DB) []job {
	t.Helper()

	rows, err := db.Query(t.Context(), `SELECT queue, CASE
		WHEN state IN ('available', 'retryable') AND coalesce(array_length(errors, 1), 0) > 0 THEN 'retrying'
		ELSE state::text END FROM river_job ORDER BY id`)
	require.NoError(t, err)

	all, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (job, error) {
		var j job
		err := row.Scan(&j.Queue, &j.State)
		if j.State == string(queue.StateCanceled) {
			j.State = "given up"
		}

		return j, err
	})
	require.NoError(t, err)

	return all
}
