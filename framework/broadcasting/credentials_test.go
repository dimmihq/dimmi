package broadcasting_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
The answers a test credential gives when asked whether it is alive.
*/
const (
	credentialAlive int32 = iota
	credentialDead
	credentialUnknown
)

/*
credentialResolver authenticates a token "user/credential", whose
credential answers as the state says.
*/
func credentialResolver(state *atomic.Int32) broadcasting.Resolver {
	return func(_ context.Context, _ *http.Request, token string) (broadcasting.Principal, error) {
		user, credential, _ := strings.Cut(token, "/")

		return broadcasting.Principal{
			UserID: user, Credential: credential,
			Alive: func(context.Context) (bool, error) {
				switch state.Load() {
				case credentialDead:
					return false, nil
				case credentialUnknown:
					return false, errors.New("cannot tell")
				default:
					return true, nil
				}
			},
		}, nil
	}
}

func TestDisconnectCredential(t *testing.T) {
	t.Parallel()

	t.Run("closes only the connections of that credential, on every node", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		state := &atomic.Int32{}
		cfg := broadcasting.Config{Resolve: credentialResolver(state)}
		first, second := nodeWith(t, client, cfg), nodeWith(t, client, cfg)
		here, there := connect(t, first, "u1/laptop"), connect(t, second, "u1/laptop")
		kept := connect(t, first, "u1/phone")
		stranger := connect(t, first, "u2/laptop")

		require.NoError(t, first.server.DisconnectCredential(t.Context(), "u1", "laptop"))

		assert.Equal(t, websocket.StatusCode(4009), here.closed())
		assert.Equal(t, websocket.StatusCode(4009), there.closed())
		assert.Nil(t, kept.subscribe("room:1", "", 0).Error)
		assert.Nil(t, stranger.subscribe("room:1", "", 0).Error)
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, n.server.DisconnectCredential(ctx, "u1", "laptop"), context.Canceled)
	})
}

func TestCredentialChecks(t *testing.T) {
	t.Parallel()

	t.Run("closes a connection asking for a channel once its credential is dead", func(t *testing.T) {
		t.Parallel()
		state := &atomic.Int32{}
		n := nodeWith(t, testkit.NewValkey(t), broadcasting.Config{Resolve: credentialResolver(state)})
		c := connect(t, n, "u1/laptop")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		state.Store(credentialDead)
		c.send(map[string]any{"subscribe": map[string]any{"channel": "room:2"}})

		assert.Equal(t, websocket.StatusCode(3005), c.closed())
	})

	t.Run("refuses a channel, keeping the connection, when the credential cannot be asked", func(t *testing.T) {
		t.Parallel()
		state := &atomic.Int32{}
		n := nodeWith(t, testkit.NewValkey(t), broadcasting.Config{Resolve: credentialResolver(state)})
		c := connect(t, n, "u1/laptop")

		state.Store(credentialUnknown)
		require.NotNil(t, c.subscribe("room:1", "", 0).Error)

		state.Store(credentialAlive)
		assert.Nil(t, c.subscribe("room:1", "", 0).Error)
		assert.Contains(t, n.logs.String(), "check realtime credential")
	})

	t.Run("closes a connection on the timer once its credential is dead", func(t *testing.T) {
		t.Parallel()
		state := &atomic.Int32{}
		n := nodeWith(t, testkit.NewValkey(t), broadcasting.Config{
			Resolve: credentialResolver(state), Recheck: time.Second,
		})
		c := connect(t, n, "u1/laptop")

		state.Store(credentialDead)

		assert.Equal(t, websocket.StatusCode(3005), c.closed())
	})

	t.Run("keeps a connection through a check that could not be answered", func(t *testing.T) {
		t.Parallel()
		state := &atomic.Int32{}
		state.Store(credentialUnknown)
		n := nodeWith(t, testkit.NewValkey(t), broadcasting.Config{
			Resolve: credentialResolver(state), Recheck: time.Second,
		})
		c := connect(t, n, "u1/laptop")

		assert.Never(t, func() bool { return false }, 2500*time.Millisecond, 100*time.Millisecond)

		state.Store(credentialAlive)
		assert.Nil(t, c.subscribe("room:1", "", 0).Error)
		assert.Contains(t, n.logs.String(), "check realtime credential")
	})
}
