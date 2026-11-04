package mailbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
poller is a source fed some other way than webhooks.
*/
type poller struct{}

func (poller) Name() string { return "poller" }

/*
hook is a source fed by webhooks, answering every body with one
envelope.
*/
type hook struct{ name string }

func (h hook) Name() string { return h.name }

func (h hook) Receive(context.Context, mailbox.Delivery) (*mailbox.Envelope, error) {
	return &mailbox.Envelope{Source: h.name}, nil
}

func (h hook) MaxWebhookBytes() int64 { return 1 }

func newManager() *mailbox.Manager {
	m := mailbox.NewManager(mailbox.Config{
		Default: "inbound",
		Sources: map[string]config.Section{
			"inbound": {"driver": "hook"},
			"polled":  {"driver": "poller"},
		},
	})
	m.Extend("hook", func(name string, _ config.Section) (mailbox.Source, error) { return hook{name: name}, nil })
	m.Extend("poller", func(string, config.Section) (mailbox.Source, error) { return poller{}, nil })

	return m
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("resolves sources by name and by default", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		polled, err := m.Source("polled")
		require.NoError(t, err)
		def, err := m.Default()
		require.NoError(t, err)

		assert.Equal(t, "poller", polled.Name())
		assert.Equal(t, "inbound", def.Name())
	})

	t.Run("resolves a webhook source as a receiver", func(t *testing.T) {
		t.Parallel()

		receiver, err := newManager().Receiver("inbound")
		require.NoError(t, err)

		env, err := receiver.Receive(t.Context(), mailbox.Delivery{})
		require.NoError(t, err)
		assert.Equal(t, "inbound", env.Source)
	})

	t.Run("refuses a source that takes no webhooks as a receiver", func(t *testing.T) {
		t.Parallel()

		_, err := newManager().Receiver("polled")

		require.ErrorIs(t, err, mailbox.ErrNotReceiver)
	})

	t.Run("reports a source that is not configured", func(t *testing.T) {
		t.Parallel()

		_, err := newManager().Receiver("missing")

		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})
}
