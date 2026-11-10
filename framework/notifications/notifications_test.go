package notifications_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/queue"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("registers the delivery worker once per queue", func(t *testing.T) {
		t.Parallel()

		q, err := queue.New(t.Context(), database.Config{URL: "postgres://localhost/app"}, queue.Config{})
		require.NoError(t, err)
		t.Cleanup(q.Close)

		_, err = notifications.New(channelConfig(), q)
		require.NoError(t, err)

		_, err = notifications.New(channelConfig(), q)
		require.ErrorContains(t, err, "deliver_notification")
	})
}

func TestSend(t *testing.T) {
	t.Parallel()

	ada, bo := user{ID: "ada"}, user{ID: "bo"}

	t.Run("delivers on every channel it picks with one id per recipient", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)

		require.NoError(t, m.Send(t.Context(), greeting{Channels: []string{"feed", "pager"}}, ada, bo))

		got := rec.all()
		require.Len(t, got, 4)
		assert.Equal(t, sent{ID: got[0].ID, Channel: "feed", To: "ada", Type: "greeting", Route: "feed:ada"}, got[0])
		assert.Equal(t, sent{ID: got[0].ID, Channel: "pager", To: "ada", Type: "greeting", Route: "pager:ada"}, got[1])
		assert.Equal(t, got[2].ID, got[3].ID)
		assert.NotEqual(t, got[0].ID, got[2].ID)
	})

	t.Run("skips a delivery the notification declines", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)

		require.NoError(t, m.Send(t.Context(), reminder{Skip: []string{"pager"}}, ada))

		got := rec.all()
		require.Len(t, got, 1)
		assert.Equal(t, "feed", got[0].Channel)
	})

	t.Run("keeps delivering after a failure and returns every failure", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)
		rec.failFor("ada", errors.New("mailbox full"))

		err := m.Send(t.Context(), greeting{Channels: []string{"feed", "pager"}}, ada, bo)

		require.ErrorContains(t, err, "send greeting via feed to user ada: mailbox full")
		require.ErrorContains(t, err, "send greeting via pager to user ada: mailbox full")
		assert.Len(t, rec.all(), 2)
	})

	t.Run("reports a channel that is not configured", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t, nil)

		err := m.Send(t.Context(), greeting{Channels: []string{"carrier_pigeon"}}, ada)

		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})

	t.Run("delivers on a channel the application added in code", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t, nil)
		added := newRecorder()
		m.Add("carrier_pigeon", added)

		require.NoError(t, m.Send(t.Context(), greeting{Channels: []string{"carrier_pigeon"}}, ada))

		got := added.all()
		require.Len(t, got, 1)
		assert.Equal(t, "carrier_pigeon", got[0].Channel)
	})

	t.Run("reports a failed check before sending", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)

		err := m.Send(t.Context(), reminder{Fail: true}, ada)

		require.ErrorContains(t, err, "check reminder via feed: preferences unavailable")
		assert.Empty(t, rec.all())
	})

	t.Run("cannot queue without a queue", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t, nil)

		err := m.SendTx(t.Context(), nil, greeting{}, ada)

		require.ErrorIs(t, err, notifications.ErrNoQueue)
	})
}

func TestSendTx(t *testing.T) {
	t.Parallel()

	ada, bo := user{ID: "ada"}, user{ID: "bo"}

	t.Run("delivers once the transaction commits", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, rec := newManager(t, q, ada, bo)

		require.NoError(t, db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return m.SendTx(ctx, tx, greeting{Text: "hi", Channels: []string{"feed", "pager"}}, ada, bo)
		}))
		require.NoError(t, q.Drain(t.Context()))

		got := rec.all()
		require.Len(t, got, 4)
		assert.ElementsMatch(t, []string{"feed:ada", "pager:ada", "feed:bo", "pager:bo"},
			[]any{got[0].Route, got[1].Route, got[2].Route, got[3].Route})
		assert.Equal(t, []job{
			{"default", "completed"},
			{"default", "completed"},
			{"default", "completed"},
			{"default", "completed"},
		}, jobs(t, db))
	})

	t.Run("delivers nothing when the transaction rolls back", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, rec := newManager(t, q, ada)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, m.SendTx(ctx, tx, greeting{Channels: []string{"feed"}}, ada))

			return errors.New("action failed")
		})
		require.Error(t, err)
		require.NoError(t, q.Drain(t.Context()))

		assert.Empty(t, rec.all())
		assert.Empty(t, jobs(t, db))
	})

	t.Run("pushes on its own without a transaction", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		m, rec := newManager(t, q, ada)

		require.NoError(t, m.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, q.Drain(t.Context()))

		assert.Len(t, rec.all(), 1)
	})

	t.Run("uses the options the notification picks", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, rec := newManager(t, q, ada)
		later := clock.System().Now().Add(time.Hour)

		require.NoError(t, m.SendTx(t.Context(), nil, reminder{Queue: "later", At: later}, ada))
		require.NoError(t, q.Drain(t.Context()))

		assert.Empty(t, rec.all())
		assert.Equal(t, []job{{"later", "scheduled"}, {"later", "scheduled"}}, jobs(t, db))
	})

	t.Run("checks whether to send when the delivery runs", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		m, rec := newManager(t, q, ada)

		require.NoError(t, m.SendTx(t.Context(), nil, reminder{Skip: []string{"feed"}}, ada))
		require.NoError(t, q.Drain(t.Context()))

		got := rec.all()
		require.Len(t, got, 1)
		assert.Equal(t, "pager", got[0].Channel)
	})

	t.Run("refuses what it could not deliver later", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		m, _ := newManager(t, q, ada)

		err := m.SendTx(t.Context(), nil, unencodable{}, ada)
		require.ErrorIs(t, err, notifications.ErrUnregistered)

		notifications.Register[unencodable](m)
		err = m.SendTx(t.Context(), nil, unencodable{}, ada)
		require.ErrorContains(t, err, "encode unencodable")

		err = m.SendTx(t.Context(), nil, greeting{}, robot{})
		require.ErrorIs(t, err, notifications.ErrNoResolver)

		err = m.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada)
		require.NoError(t, err)
	})

	t.Run("reports a failed push", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		m, _ := newManager(t, q, ada)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := m.SendTx(ctx, nil, greeting{Channels: []string{"feed"}}, ada)

		require.ErrorContains(t, err, "queue greeting via feed")
	})
}

func TestDeliveryJob(t *testing.T) {
	t.Parallel()

	ada := user{ID: "ada"}

	cases := []struct {
		name  string
		fail  error
		state string
	}{
		{
			"gives up a failure that can never succeed", fmt.Errorf("rejected: %w", notifications.ErrPermanent),
			"given up",
		},
		{"gives up a channel the notification does not support", notifications.ErrUnsupported, "given up"},
		{"gives up a route the channel cannot read", notifications.ErrInvalidRoute, "given up"},
		{"retries a failure that may pass", errors.New("connection reset"), "retrying"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, db := setup(t)
			m, rec := newManager(t, q, ada)
			rec.failFor("ada", tc.fail)

			require.NoError(t, m.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
			require.NoError(t, q.Drain(t.Context()))

			assert.Equal(t, []job{{"default", tc.state}}, jobs(t, db))
		})
	}

	t.Run("drops the delivery of a recipient that is gone", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, rec := newManager(t, q)
		m.Resolve("user", func(context.Context, string) (notifications.Notifiable, error) {
			return nil, notifications.ErrNotifiableGone
		})

		require.NoError(t, m.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, q.Drain(t.Context()))

		assert.Empty(t, rec.all())
		assert.Equal(t, []job{{"default", "completed"}}, jobs(t, db))
	})

	t.Run("retries when the recipient cannot be loaded", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, _ := newManager(t, q)
		m.Resolve("user", func(context.Context, string) (notifications.Notifiable, error) {
			return nil, errors.New("database unavailable")
		})

		require.NoError(t, m.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, q.Drain(t.Context()))

		assert.Equal(t, []job{{"default", "retrying"}}, jobs(t, db))
	})

	t.Run("gives up a notification that no longer decodes", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, _ := newManager(t, q, ada)

		require.NoError(t, m.SendTx(t.Context(), nil, brittle{Text: "hi"}, ada))
		require.NoError(t, q.Drain(t.Context()))

		assert.Equal(t, []job{{"default", "given up"}}, jobs(t, db))
	})

	t.Run("retries on a node that does not know the type or recipient yet", func(t *testing.T) {
		t.Parallel()
		cfg, db := migrated(t)
		sending, working := newQueue(t, cfg), newQueue(t, cfg)
		sender, _ := newManager(t, sending, ada)
		worker, err := notifications.New(channelConfig(), working)
		require.NoError(t, err)

		require.NoError(t, sender.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, working.Drain(t.Context()))
		notifications.Register[greeting](worker)
		require.NoError(t, sender.SendTx(t.Context(), nil, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, working.Drain(t.Context()))

		assert.Equal(t, []job{{"default", "retrying"}, {"default", "retrying"}}, jobs(t, db))
	})
}

/*
robot is a recipient type nobody registered a resolver for.
*/
type robot struct{}

func (robot) NotifiableType() string { return "robot" }

func (robot) NotifiableID() string { return "r2" }

func (robot) RouteFor(string) any { return nil }
