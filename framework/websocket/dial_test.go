package websocket_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coder "github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/websocket"
)

/*
echoServer echoes every message back. A message "close" makes it close
the connection with code 4000. A handshake carrying an X-Token header
gets its value as the first message.
*/
func echoServer(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coder.Accept(w, r, &coder.AcceptOptions{Subprotocols: []string{"chat.v1"}})
		if err != nil {
			return
		}

		defer func() { _ = conn.CloseNow() }()

		if token := r.Header.Get("X-Token"); token != "" {
			_ = conn.Write(r.Context(), coder.MessageText, []byte(token))
		}

		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}

			if string(data) == "close" {
				_ = conn.Close(4000, "bye")

				return
			}

			if err := conn.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

/*
dial connects to the URL through a client that allows loopback.
*/
func dial(t *testing.T, rawURL string, opts websocket.Options) *websocket.Conn {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	opts.HTTP = client

	conn, err := websocket.Dial(t.Context(), rawURL, opts)
	require.NoError(t, err)

	return conn
}

func TestDial(t *testing.T) {
	t.Parallel()

	t.Run("exchanges text and binary messages", func(t *testing.T) {
		t.Parallel()
		conn := dial(t, echoServer(t), websocket.Options{Subprotocols: []string{"chat.v1"}})
		t.Cleanup(func() { _ = conn.Close() })

		require.NoError(t, conn.Write(t.Context(), websocket.Text, []byte("hello")))
		typ, data, err := conn.Read(t.Context())
		require.NoError(t, err)
		assert.Equal(t, websocket.Text, typ)
		assert.Equal(t, "hello", string(data))

		require.NoError(t, conn.Write(t.Context(), websocket.Binary, []byte{1, 2}))
		typ, data, err = conn.Read(t.Context())
		require.NoError(t, err)
		assert.Equal(t, websocket.Binary, typ)
		assert.Equal(t, []byte{1, 2}, data)

		assert.Equal(t, "chat.v1", conn.Subprotocol())
	})

	t.Run("reports the close code the peer sent", func(t *testing.T) {
		t.Parallel()
		conn := dial(t, echoServer(t), websocket.Options{})

		require.NoError(t, conn.Write(t.Context(), websocket.Text, []byte("close")))
		_, _, err := conn.Read(t.Context())

		var closed *websocket.CloseError
		require.ErrorAs(t, err, &closed)
		assert.Equal(t, 4000, closed.Code)
		assert.Equal(t, "bye", closed.Reason)
		assert.Contains(t, closed.Error(), "4000")
	})

	t.Run("refuses a message over the read limit", func(t *testing.T) {
		t.Parallel()
		conn := dial(t, echoServer(t), websocket.Options{ReadLimit: 4})

		require.NoError(t, conn.Write(t.Context(), websocket.Text, []byte("too long")))
		_, _, err := conn.Read(t.Context())

		require.ErrorContains(t, err, "read websocket message")
	})

	t.Run("refuses private addresses unless allowed", func(t *testing.T) {
		t.Parallel()

		_, err := websocket.Dial(t.Context(), echoServer(t), websocket.Options{})

		require.ErrorIs(t, err, safedial.ErrBlocked)
	})

	t.Run("sends the handshake headers", func(t *testing.T) {
		t.Parallel()
		conn := dial(t, echoServer(t), websocket.Options{Header: http.Header{"X-Token": []string{"secret"}}})
		t.Cleanup(func() { _ = conn.Close() })

		_, data, err := conn.Read(t.Context())

		require.NoError(t, err)
		assert.Equal(t, "secret", string(data))
	})

	t.Run("rejects a malformed url", func(t *testing.T) {
		t.Parallel()

		_, err := websocket.Dial(t.Context(), "ws://bad host/", websocket.Options{})

		require.ErrorContains(t, err, "dial websocket")
	})

	t.Run("fails to write after closing", func(t *testing.T) {
		t.Parallel()
		conn := dial(t, echoServer(t), websocket.Options{})
		require.NoError(t, conn.Close())

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		require.ErrorContains(t, conn.Write(ctx, websocket.Text, []byte("x")), "write websocket message")
		require.ErrorContains(t, conn.Close(), "close websocket")
	})
}
