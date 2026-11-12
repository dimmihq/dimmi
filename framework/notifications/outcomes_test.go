package notifications_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
hushed declines the pager with a reason.
*/
type hushed struct{}

func (hushed) Type() string { return "hushed" }

func (hushed) Via(notifications.Notifiable) []string { return []string{"feed", "pager"} }

func (hushed) ShouldSend(_ context.Context, d notifications.Delivery) (bool, error) {
	if d.Channel == "pager" {
		return false, notifications.Skip("quiet_hours")
	}

	return true, nil
}

/*
outcomes collects what an observer heard.
*/
type outcomes struct {
	mu  sync.Mutex
	got []notifications.Outcome
}

func (o *outcomes) observe(_ context.Context, out notifications.Outcome) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.got = append(o.got, out)
}

func TestOutcomes(t *testing.T) {
	t.Parallel()

	ada := user{ID: "ada"}

	t.Run("reports a sent delivery and a skip with its reason", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)
		var heard outcomes
		m.Observe(heard.observe)

		require.NoError(t, m.Send(t.Context(), hushed{}, ada))

		require.Len(t, rec.all(), 1)
		require.Len(t, heard.got, 2)
		assert.Equal(t, notifications.StatusSent, heard.got[0].Status)
		assert.Equal(t, "feed", heard.got[0].Delivery.Channel)
		assert.Equal(t, notifications.StatusSkipped, heard.got[1].Status)
		assert.Equal(t, "quiet_hours", heard.got[1].Reason)
	})

	t.Run("reports a skip without a reason and a failure with its error", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)
		rec.failFor("ada", errors.New("mailbox full"))
		var heard outcomes
		m.Observe(heard.observe)

		require.Error(t, m.Send(t.Context(), reminder{Skip: []string{"pager"}}, ada))

		require.Len(t, heard.got, 2)
		assert.Equal(t, notifications.StatusFailed, heard.got[0].Status)
		require.ErrorContains(t, heard.got[0].Err, "mailbox full")
		assert.Equal(t, notifications.StatusSkipped, heard.got[1].Status)
		assert.Empty(t, heard.got[1].Reason)
	})

	t.Run("uses the context's delivery id for a single recipient only", func(t *testing.T) {
		t.Parallel()
		m, rec := newManager(t, nil)
		ctx := notifications.WithDeliveryID(t.Context(), "fixed")

		require.NoError(t, m.Send(ctx, greeting{Channels: []string{"feed"}}, ada))
		require.NoError(t, m.Send(ctx, greeting{Channels: []string{"feed"}}, ada, user{ID: "bo"}))

		got := rec.all()
		require.Len(t, got, 3)
		assert.Equal(t, "fixed", got[0].ID)
		assert.NotEqual(t, "fixed", got[1].ID)
		assert.Equal(t, "fixed", notifications.DeliveryID(ctx))
	})
}
