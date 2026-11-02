package broadcasting_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestUniSSE(t *testing.T) {
	t.Parallel()

	t.Run("streams broadcasts on the principal's channels", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := listen(t, n, map[string]any{"token": "u1|user:u1"})

		hello := c.push().Connect
		require.NotNil(t, hello)
		require.Contains(t, hello.Subs, "user:u1")

		require.NoError(t, n.server.Broadcast(t.Context(), "user:u1", "note.added", map[string]int{"n": 1}))

		p := c.push()
		assert.Equal(t, "user:u1", p.Channel)
		assert.Equal(t, "note.added", decode(t, p.Pub).Event)
	})

	t.Run("recovers from the positions in the connect command", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		first := listen(t, n, map[string]any{"token": "u1|user:u1"})
		position := first.push().Connect.Subs["user:u1"]

		require.NoError(t, n.server.Broadcast(t.Context(), "user:u1", "missed", nil))

		again := listen(t, n, map[string]any{
			"token": "u1|user:u1",
			"subs": map[string]any{
				"user:u1": map[string]any{"recover": true, "epoch": position.Epoch, "offset": position.Offset},
			},
		})
		sub := again.push().Connect.Subs["user:u1"]
		assert.True(t, sub.Recovered)
		require.Len(t, sub.Publications, 1)
		assert.Equal(t, "missed", decode(t, &sub.Publications[0]).Event)
	})

	t.Run("ends a refused connection with a disconnect push", func(t *testing.T) {
		t.Parallel()
		c := listen(t, newNode(t, testkit.NewValkey(t)), nil)

		p := c.push()

		require.NotNil(t, p.Disconnect)
		assert.Equal(t, 3500, p.Disconnect.Code)
	})

	t.Run("ends with the reason when the user is disconnected", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := listen(t, n, map[string]any{"token": "u1"})
		require.NotNil(t, c.push().Connect)

		require.NoError(t, n.server.Disconnect(t.Context(), "u1"))

		assert.Equal(t, 4009, c.push().Disconnect.Code)
	})

	t.Run("rejects a malformed connect command", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		for _, connect := range []map[string]any{{"token": 42}, {"token": strings.Repeat("x", 20_000)}} {
			resp := openSSE(t, n, connect)
			assert.Equal(t, http.StatusBadRequest, resp.Status)
		}
	})
}
