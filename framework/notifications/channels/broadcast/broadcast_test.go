package broadcast_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/broadcast"
)

/*
published is one broadcast the broadcaster received.
*/
type published struct {
	Channels []string
	Event    string
	Payload  any
}

/*
broadcaster records broadcasts, or fails them with err.
*/
type broadcaster struct {
	mu   sync.Mutex
	sent []published
	err  error
}

func (b *broadcaster) Broadcast(ctx context.Context, channel, event string, payload any) error {
	return b.BroadcastMany(ctx, []string{channel}, event, payload)
}

func (b *broadcaster) BroadcastMany(_ context.Context, channels []string, event string, payload any) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.err != nil {
		return b.err
	}

	b.sent = append(b.sent, published{Channels: channels, Event: event, Payload: payload})

	return nil
}

func (b *broadcaster) BroadcastEphemeral(ctx context.Context, channel, event string, payload any) error {
	return b.Broadcast(ctx, channel, event, payload)
}

func (b *broadcaster) all() []published {
	b.mu.Lock()
	defer b.mu.Unlock()

	return slices.Clone(b.sent)
}

type member struct {
	route any
}

func (member) NotifiableType() string { return "member" }

func (member) NotifiableID() string { return "m1" }

func (m member) RouteFor(string) any { return m.route }

type assigned struct {
	Event string
	Fail  bool
}

func (assigned) Type() string { return "assigned" }

func (assigned) Via(notifications.Notifiable) []string { return []string{"broadcast"} }

func (n assigned) ToBroadcast(context.Context, notifications.Delivery) (broadcast.Message, error) {
	if n.Fail {
		return broadcast.Message{}, errors.New("record missing")
	}

	return broadcast.Message{Event: n.Event, Data: map[string]string{"order": "o1"}}, nil
}

/*
silent has no broadcast rendering.
*/
type silent struct{}

func (silent) Type() string { return "silent" }

func (silent) Via(notifications.Notifiable) []string { return []string{"broadcast"} }

/*
setup returns a manager whose broadcast channel publishes through b.
*/
func setup(t *testing.T, b broadcasting.Broadcaster) *notifications.Manager {
	t.Helper()

	m, err := notifications.New(notifications.Config{
		Channels: map[string]config.Section{"broadcast": {"driver": "broadcast"}},
	}, nil)
	require.NoError(t, err)
	m.Extend("broadcast", broadcast.Factory(b))

	return m
}

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("publishes the id, type and data on the recipient's channel", func(t *testing.T) {
		t.Parallel()
		b := &broadcaster{}

		require.NoError(t, setup(t, b).Send(t.Context(), assigned{}, member{route: "member.m1"}))

		sent := b.all()
		require.Len(t, sent, 1)
		assert.Equal(t, []string{"member.m1"}, sent[0].Channels)
		assert.Equal(t, "assigned", sent[0].Event)
		encoded, err := json.Marshal(sent[0].Payload)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(encoded, &payload))
		assert.NotEmpty(t, payload["id"])
		assert.Equal(t, "assigned", payload["type"])
		assert.Equal(t, map[string]any{"order": "o1"}, payload["data"])
	})

	t.Run("names the event as the notification asks", func(t *testing.T) {
		t.Parallel()
		b := &broadcaster{}

		require.NoError(t, setup(t, b).Send(t.Context(), assigned{Event: "notification.created"},
			member{route: []string{"member.m1", "", "team.t1"}}))

		sent := b.all()
		require.Len(t, sent, 1)
		assert.Equal(t, "notification.created", sent[0].Event)
		assert.Equal(t, []string{"member.m1", "team.t1"}, sent[0].Channels)
	})

	t.Run("skips a recipient without a channel", func(t *testing.T) {
		t.Parallel()
		b := &broadcaster{}

		require.NoError(t, setup(t, b).Send(t.Context(), assigned{}, member{}, member{route: ""}))

		assert.Empty(t, b.all())
	})

	t.Run("refuses what it cannot publish", func(t *testing.T) {
		t.Parallel()
		m := setup(t, &broadcaster{})

		require.ErrorIs(t, m.Send(t.Context(), assigned{}, member{route: 7}), notifications.ErrInvalidRoute)
		require.ErrorIs(t, m.Send(t.Context(), silent{}, member{route: "a"}), notifications.ErrUnsupported)
		require.ErrorContains(t, m.Send(t.Context(), assigned{Fail: true}, member{route: "a"}),
			"render assigned for broadcast: record missing")
	})

	t.Run("marks a broadcast that can never be published as permanent", func(t *testing.T) {
		t.Parallel()

		for _, cause := range []error{broadcasting.ErrUnknownChannel, broadcasting.ErrPayloadTooLarge} {
			err := setup(t, &broadcaster{err: cause}).Send(t.Context(), assigned{}, member{route: "a"})

			require.ErrorIs(t, err, notifications.ErrPermanent)
			require.ErrorIs(t, err, cause)
		}
	})

	t.Run("leaves other failures retryable", func(t *testing.T) {
		t.Parallel()

		err := setup(t, &broadcaster{err: errors.New("valkey down")}).Send(t.Context(), assigned{}, member{route: "a"})

		require.ErrorContains(t, err, "broadcast assigned: valkey down")
		require.NotErrorIs(t, err, notifications.ErrPermanent)
	})
}
