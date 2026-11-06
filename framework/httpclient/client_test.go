package httpclient_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/drivertest"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
)

func TestContract(t *testing.T) {
	t.Parallel()

	t.Run("real server", func(t *testing.T) {
		t.Parallel()
		drivertest.Run(t, realClient)
	})

	t.Run("fake", func(t *testing.T) {
		t.Parallel()
		drivertest.Run(t, fakeClient)
	})
}

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("sends the method, headers, token and json body", func(t *testing.T) {
		t.Parallel()
		got := make(chan *http.Request, 1)
		bodies := make(chan string, 1)
		server, client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			got <- r
			bodies <- string(body)
			w.WriteHeader(http.StatusAccepted)
		}))

		resp, err := client.New().
			WithToken("t0k").
			WithHeader("X-Signature", "sig").
			Accept("application/json").
			Patch(t.Context(), server.URL+"/items/1", map[string]int{"n": 1})
		require.NoError(t, err)

		req := <-got
		assert.Equal(t, http.StatusAccepted, resp.Status)
		assert.Equal(t, http.MethodPatch, req.Method)
		assert.Equal(t, "Bearer t0k", req.Header.Get("Authorization"))
		assert.Equal(t, "sig", req.Header.Get("X-Signature"))
		assert.Equal(t, "application/json", req.Header.Get("Accept"))
		assert.Contains(t, req.Header.Get("Content-Type"), "application/json")
		assert.JSONEq(t, `{"n":1}`, <-bodies)
	})

	t.Run("sends every method", func(t *testing.T) {
		t.Parallel()
		methods := make(chan string, 4)
		server, client := serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			methods <- r.Method
		}))
		ctx, target := t.Context(), server.URL

		_, err := client.New().Get(ctx, target)
		require.NoError(t, err)
		_, err = client.New().Delete(ctx, target)
		require.NoError(t, err)
		_, err = client.New().Put(ctx, target, "raw")
		require.NoError(t, err)
		_, err = client.New().Post(ctx, target, nil)
		require.NoError(t, err)

		want := []string{http.MethodGet, http.MethodDelete, http.MethodPut, http.MethodPost}
		assert.Equal(t, want, []string{<-methods, <-methods, <-methods, <-methods})
	})

	t.Run("accepts a body that fills the cap exactly", func(t *testing.T) {
		t.Parallel()
		server, client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "four")
		}))

		resp, err := client.New().MaxBody(4).Get(t.Context(), server.URL)

		require.NoError(t, err)
		assert.Equal(t, "four", string(resp.Body))
		assert.Equal(t, "four", readAll(t, resp.Reader()))
		require.NoError(t, resp.Close())
	})

	t.Run("times out a slow response", func(t *testing.T) {
		t.Parallel()
		server, client := serve(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-clock.System().After(time.Second):
			}
		}))

		_, err := client.New().Timeout(50*time.Millisecond).Get(t.Context(), server.URL)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deadline exceeded")
	})

	t.Run("keeps a streamed body open past the cap", func(t *testing.T) {
		t.Parallel()
		server, client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", 100))
		}))

		resp, err := client.New().MaxBody(10).Stream().Get(t.Context(), server.URL)
		require.NoError(t, err)

		assert.Len(t, readAll(t, resp.Reader()), 100)
		require.NoError(t, resp.Close())
	})

	t.Run("refuses a redirect to a host that is not allowed", func(t *testing.T) {
		t.Parallel()
		private := httpTestServer(t)
		server, client := serve(t, http.RedirectHandler(private+"/secret", http.StatusFound))

		_, err := client.New().Get(t.Context(), server.URL)

		require.ErrorIs(t, err, httpclient.ErrBlockedHost)
	})

	t.Run("refuses a redirect to another scheme", func(t *testing.T) {
		t.Parallel()
		server, client := serve(t, http.RedirectHandler("ftp://files.example.test/x", http.StatusFound))

		_, err := client.New().Get(t.Context(), server.URL)

		require.ErrorIs(t, err, httpclient.ErrBlockedHost)
	})

	t.Run("reaches private hosts when private networks are allowed", func(t *testing.T) {
		t.Parallel()
		target := httpTestServer(t)
		client := newClient(t, httpclient.Config{AllowPrivateNetworks: true})

		resp, err := client.New().Get(t.Context(), target)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status)
	})

	t.Run("rejects a url it cannot parse", func(t *testing.T) {
		t.Parallel()

		_, err := newClient(t, httpclient.Config{}).New().Get(t.Context(), "http://bad host/")

		require.Error(t, err)
	})

	t.Run("reports a body that cannot be decoded", func(t *testing.T) {
		t.Parallel()

		resp := &httpclient.Response{Body: []byte("not json")}

		require.Error(t, resp.JSON(&struct{}{}))
	})
}

func TestStdClient(t *testing.T) {
	t.Parallel()

	t.Run("reaches allowed hosts and refuses the rest", func(t *testing.T) {
		t.Parallel()
		server, client := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		other := httpTestServer(t)

		ok, err := stdGet(t, client, server.URL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, ok)

		_, err = stdGet(t, client, other)
		require.ErrorIs(t, err, safedial.ErrBlocked)
	})

	t.Run("answers from the fake once faked", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{})
		httpclienttest.New(client).Stub("https://api.example.test/*", httpclient.Response{Status: http.StatusTeapot})

		status, err := stdGet(t, client, "https://api.example.test/v1/models")

		require.NoError(t, err)
		assert.Equal(t, http.StatusTeapot, status)
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("refuses a fake unless fakes are allowed", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		assert.PanicsWithValue(t, httpclient.ErrFakesNotAllowed, func() { httpclienttest.New(client) })
	})

	t.Run("trusts plaintext only for listed hosts or private addresses", func(t *testing.T) {
		t.Parallel()
		listed, err := httpclient.New(httpclient.Config{AllowHosts: []string{"ollama.lan:11434"}})
		require.NoError(t, err)
		private, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)

		assert.True(t, listed.AllowsPlaintext("ollama.lan:11434"))
		assert.False(t, listed.AllowsPlaintext("localhost:11434"))
		assert.False(t, listed.AllowsPlaintext("ollama.lan"))
		assert.True(t, private.AllowsPlaintext("localhost:11434"))
		assert.True(t, private.AllowsPlaintext("10.0.0.4:8000"))
		assert.True(t, private.AllowsPlaintext("[::1]:8000"))
		assert.False(t, private.AllowsPlaintext("example.com:80"))
	})

	t.Run("rejects an allowed host without a port", func(t *testing.T) {
		t.Parallel()

		_, err := httpclient.New(httpclient.Config{AllowHosts: []string{"localhost"}})

		require.ErrorIs(t, err, safedial.ErrInvalidHost)
	})
}

/*
httpTestServer starts a loopback server that no client allows and
returns its URL.
*/
func httpTestServer(t *testing.T) string {
	t.Helper()

	server, _ := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	return server.URL
}

func stdGet(t *testing.T, client *httpclient.Client, target string) (int, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	require.NoError(t, err)

	resp, err := client.StdClient().Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode, nil
}

func readAll(t *testing.T, r io.Reader) string {
	t.Helper()

	data, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(data)
}
