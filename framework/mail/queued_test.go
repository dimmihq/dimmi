package mail_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
invitation is a mailable whose data survives the queue.
*/
type invitation struct {
	Inviter string `json:"inviter"`
	URL     string `json:"url"`
}

func (i invitation) Envelope() mail.Envelope {
	return mail.Envelope{Subject: i.Inviter + " invited you"}
}

func (i invitation) Content(context.Context) (mail.Content, error) {
	return mail.Content{Text: "Accept: " + i.URL}, nil
}

/*
failing is a driver whose every send fails with the error its section
names.
*/
type failing struct{ err error }

func (f failing) Send(context.Context, mail.Message) (mail.SendResult, error) {
	return mail.SendResult{}, f.err
}

func sendErrors() map[string]error {
	return map[string]error{
		"permanent":   fmt.Errorf("550 user unknown: %w", mail.ErrPermanent),
		"temporary":   fmt.Errorf("timeout: %w", mail.ErrTemporary),
		"rate":        &mail.RateLimitedError{RetryAfter: time.Hour},
		"rate_unsaid": &mail.RateLimitedError{},
	}
}

/*
queued is a mail manager on a real queue and database.
*/
type queued struct {
	mail  *mail.Manager
	queue *queue.Queue
	db    *database.DB
}

/*
newQueued returns a manager whose default mailer fails as named, or
works when the name is empty, queueing on a fresh migrated database.
*/
func newQueued(t *testing.T, failure string) queued {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{queue.Migration()}}.Up(t.Context())
	require.NoError(t, err)
	cfg := database.Config{URL: url, PoolMode: database.PoolModeSession, MaxConns: 2}

	q, err := queue.New(t.Context(), cfg, queue.Config{
		Queues: map[string]int{"default": 2}, MaxConns: 3, PollInterval: 20 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = q.Stop(context.WithoutCancel(t.Context()))
		q.Close()
	})

	db, err := database.Open(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	m := mail.NewManager(mail.Config{
		Default:    "system",
		AllowFakes: true,
		From:       mail.Address{Email: "app@example.com"},
		Mailers:    map[string]config.Section{"system": {"driver": "failing", "fail": failure}},
		Translator: translator(t),
	})
	m.Extend("failing", func(_ string, section config.Section) (mail.Mailer, error) {
		return failing{err: sendErrors()[section.String("fail")]}, nil
	})
	mail.Register[invitation](m, "invitation")
	mail.Register[welcome](m, "welcome")
	require.NoError(t, m.UseQueue(q))

	return queued{mail: m, queue: q, db: db}
}

/*
job reads the state of the only job and when it is due.
*/
func (q queued) job(t *testing.T) (string, time.Time) {
	t.Helper()

	var (
		state string
		due   time.Time
	)
	require.NoError(t, q.db.QueryRow(t.Context(), "SELECT state, scheduled_at FROM river_job").Scan(&state, &due))

	return state, due
}

func TestQueueTx(t *testing.T) {
	t.Parallel()

	ana := mail.Address{Email: "ana@example.com"}

	t.Run("sends the mailable once the transaction commits", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()

		err := q.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return q.mail.To(ana).Cc(mail.Address{Email: "bo@example.com"}).QueueTx(ctx, tx, invitation{
				Inviter: "Ana", URL: "https://example.com/accept",
			})
		})
		require.NoError(t, err)
		fake.AssertNothingSent(t)

		require.NoError(t, q.queue.Drain(t.Context()))

		mail.AssertSent(t, fake, func(m invitation) bool { return m.Inviter == "Ana" })
		sent := fake.Sent()[0]
		assert.Equal(t, "system", sent.Mailer)
		assert.Equal(t, []mail.Address{ana}, sent.Message.To)
		assert.Equal(t, "bo@example.com", sent.Message.Cc[0].Email)
		assert.Equal(t, "Ana invited you", sent.Message.Subject)
		assert.Equal(t, "Accept: https://example.com/accept", sent.Message.Text)
	})

	t.Run("sends nothing when the transaction rolls back", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()
		errAction := errors.New("action failed")

		err := q.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, q.mail.To(ana).QueueTx(ctx, tx, invitation{Inviter: "Ana"}))

			return errAction
		})
		require.ErrorIs(t, err, errAction)

		require.NoError(t, q.queue.Drain(t.Context()))
		fake.AssertNothingSent(t)
	})

	t.Run("queues on the mailer it names", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()

		require.NoError(t, q.mail.To(ana).Mailer("customer").Queue(t.Context(), invitation{Inviter: "Cy"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		assert.Equal(t, "customer", fake.Sent()[0].Mailer)
	})

	t.Run("queues outside a transaction", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()

		require.NoError(t, q.mail.To(ana).Bcc(mail.Address{Email: "copy@example.com"}).
			Queue(t.Context(), invitation{Inviter: "Bo"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		mail.AssertSent(t, fake, func(m invitation) bool { return m.Inviter == "Bo" })
		assert.Equal(t, "copy@example.com", fake.Sent()[0].Message.Bcc[0].Email)
	})

	t.Run("sends through the real mailer when not faked", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")

		require.NoError(t, q.mail.To(ana).Queue(t.Context(), invitation{Inviter: "Ana"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		state, _ := q.job(t)
		assert.Equal(t, string(queue.StateCompleted), state)
	})
}

func TestQueuedFailures(t *testing.T) {
	t.Parallel()

	ana := mail.Address{Email: "ana@example.com"}

	t.Run("gives up a mail the provider refuses for good", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "permanent")

		require.NoError(t, q.mail.To(ana).Queue(t.Context(), invitation{Inviter: "Ana"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		state, _ := q.job(t)
		assert.Equal(t, string(queue.StateCanceled), state)
	})

	t.Run("retries a temporary failure later", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "temporary")

		require.NoError(t, q.mail.To(ana).Queue(t.Context(), invitation{Inviter: "Ana"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		var failed int
		require.NoError(t, q.db.QueryRow(t.Context(),
			"SELECT coalesce(array_length(errors, 1), 0) FROM river_job WHERE state IN ('retryable', 'available')",
		).Scan(&failed))
		assert.Equal(t, 1, failed)
	})

	t.Run("waits as long as a rate limit asks", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "rate")
		before := clock.System().Now()

		require.NoError(t, q.mail.To(ana).Queue(t.Context(), invitation{Inviter: "Ana"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		state, due := q.job(t)
		assert.Equal(t, string(queue.StateScheduled), state)
		assert.WithinDuration(t, before.Add(time.Hour), due, 10*time.Second)
	})

	t.Run("spreads the wait over a minute when the provider gives none", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "rate_unsaid")
		before := clock.System().Now()

		require.NoError(t, q.mail.To(ana).Queue(t.Context(), invitation{Inviter: "Ana"}))
		require.NoError(t, q.queue.Drain(t.Context()))

		state, due := q.job(t)
		assert.Contains(t, []string{string(queue.StateScheduled), string(queue.StateAvailable)}, state,
			"a wait shorter than the scheduler's interval is stored as available and due later")
		assert.True(t, due.After(before), "due %v not after %v", due, before)
		assert.True(t, due.Before(before.Add(time.Minute+10*time.Second)), "due %v beyond a minute", due)
	})

	jobs := map[string]mail.SendArgs{
		"gives up a job whose mailable is no longer registered": {Mailable: "gone", Payload: []byte(`{}`)},
		"gives up a job whose data no longer decodes":           {Mailable: "invitation", Payload: []byte(`"x"`)},
	}

	for name, args := range jobs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			q := newQueued(t, "")

			require.NoError(t, q.queue.Push(t.Context(), args))
			require.NoError(t, q.queue.Drain(t.Context()))

			state, _ := q.job(t)
			assert.Equal(t, string(queue.StateCanceled), state)
		})
	}
}

func TestQueueRefusals(t *testing.T) {
	t.Parallel()

	ana := mail.Address{Email: "ana@example.com"}

	t.Run("refuses to queue without a queue", func(t *testing.T) {
		t.Parallel()

		err := mail.NewManager(mail.Config{}).To(ana).Queue(t.Context(), invitation{})

		require.ErrorIs(t, err, mail.ErrNoQueue)
	})

	t.Run("refuses a mailable no one registered", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")

		err := q.mail.To(ana).Queue(t.Context(), greeting{name: "x"})

		require.ErrorIs(t, err, mail.ErrUnregisteredMailable)
	})

	t.Run("refuses a mailable that cannot be encoded", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		mail.Register[unencodable](q.mail, "unencodable")

		err := q.db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return q.mail.To(ana).QueueTx(ctx, tx, unencodable{Channel: make(chan int)})
		})

		require.Error(t, err)
	})

	t.Run("refuses to register the worker twice", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")

		require.Error(t, q.mail.UseQueue(q.queue))
	})
}

/*
unencodable is a mailable JSON cannot encode.
*/
type unencodable struct {
	Channel chan int `json:"channel"`
}

func (unencodable) Envelope() mail.Envelope { return mail.Envelope{} }

func (unencodable) Content(context.Context) (mail.Content, error) { return mail.Content{}, nil }
