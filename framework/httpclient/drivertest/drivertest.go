/*
|--------------------------------------------------------------------------
| Client Contract
|--------------------------------------------------------------------------
|
| Code tested against stubs must behave the same against real servers.
| Each case lists routes; the constructor returns a client and a base URL
| whose routes answer with those responses, whether a real server serves
| them or the fake stubs them. Both must then answer the same way,
| including the refusals.
|
|   func TestClient(t *testing.T) {
|       drivertest.Run(t, newClient)
|   }
|
*/

package drivertest

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Route is one method and path with the response it gets.
*/
type Route struct {
	Method   string
	Path     string
	Response httpclient.Response
}

/*
NewClient returns a client and the base URL that serves the routes.
*/
type NewClient func(t *testing.T, routes []Route) (*httpclient.Client, string)

/*
Routes are the routes every case reads.
*/
func Routes() []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/json", Response: httpclient.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"r1"}},
			Body:   []byte(`{"name":"ada"}`),
		}},
		{Method: http.MethodPost, Path: "/items", Response: httpclient.Response{Status: http.StatusCreated}},
		{Method: http.MethodGet, Path: "/missing", Response: httpclient.Response{Status: http.StatusNotFound}},
		{Method: http.MethodGet, Path: "/moved", Response: redirect("/json")},
		{Method: http.MethodGet, Path: "/loop", Response: redirect("/loop")},
		{Method: http.MethodGet, Path: "/events", Response: httpclient.Response{
			Status: http.StatusOK,
			Header: http.Header{"Content-Type": {"text/event-stream"}},
			Body:   []byte("event: ping\nid: 7\ndata: a\ndata: b\n\n: comment\ndata: c\n\n"),
		}},
	}
}

func redirect(location string) httpclient.Response {
	return httpclient.Response{Status: http.StatusFound, Header: http.Header{"Location": {location}}}
}

/*
Run runs the contract against clients from the constructor.
*/
func Run(t *testing.T, newClient NewClient) {
	t.Helper()

	client, base := newClient(t, Routes())

	t.Run("responses", func(t *testing.T) { t.Parallel(); runResponses(t, client, base) })
	t.Run("streams", func(t *testing.T) { t.Parallel(); runStreams(t, client, base) })
	t.Run("redirects", func(t *testing.T) { t.Parallel(); runRedirects(t, client, base) })
	t.Run("refusals", func(t *testing.T) { t.Parallel(); runRefusals(t, client) })
	t.Run("cancellation", func(t *testing.T) { t.Parallel(); runCancellation(t, client, base) })
}

func runResponses(t *testing.T, client *httpclient.Client, base string) {
	t.Helper()

	t.Run("returns the status, headers and decoded body", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().Accept("application/json").Get(t.Context(), base+"/json")
		require.NoError(t, err)

		var doc struct{ Name string }
		require.NoError(t, resp.JSON(&doc))
		assert.Equal(t, http.StatusOK, resp.Status)
		assert.Equal(t, "r1", resp.Header.Get("X-Request-Id"))
		assert.Equal(t, "ada", doc.Name)
	})

	t.Run("sends a body and returns the status it gets", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().WithToken("secret").Post(t.Context(), base+"/items", map[string]string{"a": "b"})

		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.Status)
	})

	t.Run("returns an error status as a response, not an error", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().Get(t.Context(), base+"/missing")

		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.Status)
	})

	t.Run("refuses a body over the cap", func(t *testing.T) {
		t.Parallel()

		_, err := client.New().MaxBody(4).Get(t.Context(), base+"/json")

		require.ErrorIs(t, err, httpclient.ErrTooLarge)
	})
}

func runStreams(t *testing.T, client *httpclient.Client, base string) {
	t.Helper()

	t.Run("streams server-sent events", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().Stream().Get(t.Context(), base+"/events")
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Close() })

		events := httpclient.SSE(resp)
		first, err := events.Next()
		require.NoError(t, err)
		second, err := events.Next()
		require.NoError(t, err)
		_, err = events.Next()

		require.ErrorIs(t, err, io.EOF)
		assert.Equal(t, httpclient.Event{ID: "7", Event: "ping", Data: "a\nb"}, first)
		assert.Equal(t, httpclient.Event{ID: "7", Event: "message", Data: "c"}, second)
	})
}

func runRedirects(t *testing.T, client *httpclient.Client, base string) {
	t.Helper()

	t.Run("follows a redirect", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().Get(t.Context(), base+"/moved")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status)
	})

	t.Run("returns a redirect as is when told not to follow", func(t *testing.T) {
		t.Parallel()

		resp, err := client.New().WithoutRedirects().Get(t.Context(), base+"/moved")

		require.NoError(t, err)
		assert.Equal(t, http.StatusFound, resp.Status)
		assert.Equal(t, "/json", resp.Header.Get("Location"))
	})

	t.Run("stops a redirect loop", func(t *testing.T) {
		t.Parallel()

		_, err := client.New().Get(t.Context(), base+"/loop")

		require.ErrorIs(t, err, httpclient.ErrTooManyRedirects)
	})
}

func runRefusals(t *testing.T, client *httpclient.Client) {
	t.Helper()

	blocked := []string{
		"http://127.0.0.1:1/",
		"http://localhost:1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]:1/",
		"http://10.0.0.1:1/",
		"file:///etc/passwd",
	}

	for _, target := range blocked {
		t.Run("refuses "+target, func(t *testing.T) {
			t.Parallel()

			_, err := client.New().Get(t.Context(), target)

			require.ErrorIs(t, err, httpclient.ErrBlockedHost)
		})
	}
}

func runCancellation(t *testing.T, client *httpclient.Client, base string) {
	t.Helper()

	t.Run("fails with the context's error when canceled before the call", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := client.New().Post(ctx, base+"/items", map[string]string{"a": "b"})

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fails reading a stream once the context is canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())

		resp, err := client.New().Stream().Get(ctx, base+"/events")
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Close() })
		cancel()

		_, err = io.ReadAll(resp.Reader())

		require.ErrorIs(t, err, context.Canceled)
	})
}
