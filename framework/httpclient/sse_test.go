package httpclient_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

func streamOf(t *testing.T, body string) *httpclient.SSEReader {
	t.Helper()

	client := newClient(t, httpclient.Config{})
	httpclienttest.New(client).Stub("https://stream.example.test/", httpclient.Response{Body: []byte(body)})

	resp, err := client.New().Stream().Get(t.Context(), "https://stream.example.test/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Close() })

	return httpclient.SSE(resp)
}

func TestSSE(t *testing.T) {
	t.Parallel()

	t.Run("drops an event cut off by the end of the stream", func(t *testing.T) {
		t.Parallel()
		events := streamOf(t, "data: whole\n\ndata: cut")

		first, err := events.Next()
		require.NoError(t, err)
		_, err = events.Next()

		assert.Equal(t, "whole", first.Data)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("forgets the type of an event without data", func(t *testing.T) {
		t.Parallel()
		events := streamOf(t, "event: lonely\n\ndata:x\r\n\r\n")

		got, err := events.Next()

		require.NoError(t, err)
		assert.Equal(t, httpclient.Event{Event: "message", Data: "x"}, got)
	})

	t.Run("refuses a line over the cap", func(t *testing.T) {
		t.Parallel()
		events := streamOf(t, "data: "+strings.Repeat("x", httpclient.MaxEventSize+1)+"\n\n")

		_, err := events.Next()

		require.ErrorIs(t, err, httpclient.ErrTooLarge)
	})

	t.Run("refuses event data over the cap", func(t *testing.T) {
		t.Parallel()
		line := "data: " + strings.Repeat("x", httpclient.MaxEventSize/2) + "\n"
		events := streamOf(t, line+line+line+"\n")

		_, err := events.Next()

		require.ErrorIs(t, err, httpclient.ErrTooLarge)
	})

	t.Run("reports a broken stream", func(t *testing.T) {
		t.Parallel()
		events := httpclient.SSE(&httpclient.Response{})
		server, client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "data: cut short")
		}))
		resp, err := client.New().Stream().Get(t.Context(), server.URL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Close() })

		_, err = events.Next()
		require.ErrorIs(t, err, io.EOF)
		_, err = httpclient.SSE(resp).Next()
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})
}
