package notifications_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
gated is a notification that asks a service at delivery time whether
it still goes out; the service is never encoded.
*/
type gated struct {
	Text string `json:"text"`
	open func() bool
}

func (*gated) Type() string { return "gated" }

func (*gated) Via(notifications.Notifiable) []string { return []string{"feed"} }

func (g *gated) ShouldSend(context.Context, notifications.Delivery) (bool, error) {
	return g.open(), nil
}

func TestRegisterWith(t *testing.T) {
	t.Parallel()

	t.Run("builds each queued notification with its services before decoding it", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		u := user{ID: "u1"}
		m, rec := newManager(t, q, u)
		allowed := false
		notifications.RegisterWith(m, func() *gated { return &gated{open: func() bool { return allowed }} })

		require.NoError(t, m.SendTx(t.Context(), nil, &gated{Text: "first"}, u))
		require.NoError(t, q.Drain(t.Context()))
		assert.Empty(t, rec.all())

		allowed = true
		require.NoError(t, m.SendTx(t.Context(), nil, &gated{Text: "second"}, u))
		require.NoError(t, q.Drain(t.Context()))
		require.Len(t, rec.all(), 1)
		assert.Equal(t, "gated", rec.all()[0].Type)
		assert.NotEmpty(t, jobs(t, db))
	})
}
