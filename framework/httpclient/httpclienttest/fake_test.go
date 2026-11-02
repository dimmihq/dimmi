package httpclienttest_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

func TestFake(t *testing.T) {
	t.Parallel()

	t.Run("records what was sent", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{})
		fake := httpclienttest.New(client)

		_, err := client.New().WithToken("t").Post(t.Context(), "https://hooks.example.test/in", `{"ok":true}`)
		require.NoError(t, err)

		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Method == http.MethodPost &&
				req.URL.Path == "/in" &&
				req.Header.Get("Authorization") == "Bearer t" &&
				string(req.Body) == `{"ok":true}`
		})
	})

	t.Run("answers an unmatched request with an empty success", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{})
		httpclienttest.New(client)

		resp, err := client.New().Get(t.Context(), "https://anything.example.test/")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status)
		assert.Empty(t, resp.Body)
	})

	t.Run("fails stray requests when prevented", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{})
		httpclienttest.New(client).PreventStrayRequests().Stub("POST https://api.example.test/*", httpclient.Response{})

		_, err := client.New().Get(t.Context(), "https://api.example.test/x")

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
	})

	t.Run("matches a stub without a method on any method", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{})
		httpclienttest.New(client).PreventStrayRequests().Stub("https://api.example.test/*", httpclient.Response{Body: []byte("hit")})

		resp, err := client.New().Delete(t.Context(), "https://api.example.test/items/1?force=1")

		require.NoError(t, err)
		assert.Equal(t, "hit", string(resp.Body))
	})

	t.Run("lets a fake request reach a host allowed by name", func(t *testing.T) {
		t.Parallel()
		client := newClient(t, httpclient.Config{AllowHosts: []string{"localhost:11434"}})
		httpclienttest.New(client).Stub("http://localhost:11434/*", httpclient.Response{Status: http.StatusAccepted})

		resp, err := client.New().Get(t.Context(), "http://localhost:11434/api/tags")

		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.Status)
	})

	t.Run("fails the assertion when nothing matches", func(t *testing.T) {
		t.Parallel()
		recorder := &testing.T{}

		fake := httpclienttest.New(newClient(t, httpclient.Config{}))

		fake.AssertSent(recorder, func(httpclienttest.SentRequest) bool { return true })

		assert.True(t, recorder.Failed())
	})
}

/*
newClient returns a client for the configuration that tests may fake.
*/
func newClient(t *testing.T, cfg httpclient.Config) *httpclient.Client {
	t.Helper()

	cfg.AllowFakes = true
	client, err := httpclient.New(cfg)
	require.NoError(t, err)

	return client
}
