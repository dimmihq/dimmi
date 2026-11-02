package customdomain_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

/*
probe is the probe every test uses.
*/
func probe() customdomain.Probe {
	return customdomain.Probe{Path: "/.well-known/domain-check/", Key: []byte("shared secret")}
}

/*
localClient returns an HTTP client allowed to reach test servers on
loopback addresses.
*/
func localClient(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, AllowFakes: true})
	require.NoError(t, err)

	return client
}

/*
serveProbe serves the probe handler and returns the server's host:port.
*/
func serveProbe(t *testing.T, p customdomain.Probe) string {
	t.Helper()

	server := httptest.NewServer(p.Handler())
	t.Cleanup(server.Close)

	return strings.TrimPrefix(server.URL, "http://")
}

func TestProbe(t *testing.T) {
	t.Parallel()

	t.Run("passes when the hostname reaches this application", func(t *testing.T) {
		t.Parallel()
		host := serveProbe(t, probe())

		assert.True(t, probe().Run(t.Context(), localClient(t), host, "tok3n"))
	})

	t.Run("fails when another server answers", func(t *testing.T) {
		t.Parallel()
		host := serveProbe(t, customdomain.Probe{Path: "/.well-known/domain-check", Key: []byte("other secret")})

		assert.False(t, probe().Run(t.Context(), localClient(t), host, "tok3n"))
	})

	t.Run("fails when the server echoes the token back", func(t *testing.T) {
		t.Parallel()
		client := localClient(t)
		httpclienttest.New(client).Stub("http://docs.example.com/.well-known/domain-check/tok3n",
			httpclient.Response{Status: http.StatusOK, Body: []byte("tok3n")})

		assert.False(t, probe().Run(t.Context(), client, "docs.example.com", "tok3n"))
	})

	t.Run("fails on an error status or no connection", func(t *testing.T) {
		t.Parallel()
		client := localClient(t)
		httpclienttest.New(client).
			Stub("http://down.example.com/*", httpclient.Response{Status: http.StatusBadGateway}).
			PreventStrayRequests()

		assert.False(t, probe().Run(t.Context(), client, "down.example.com", "tok3n"))
		assert.False(t, probe().Run(t.Context(), client, "unknown.example.com", "tok3n"))
	})

	t.Run("answers as plain text that is never cached", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/.well-known/domain-check/tok3n", http.NoBody)

		probe().Handler().ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		assert.Len(t, recorder.Body.String(), 64)
	})

	t.Run("answers not found outside a single token under its path", func(t *testing.T) {
		t.Parallel()

		for _, target := range []string{"/", "/.well-known/domain-check/", "/.well-known/domain-check/a/b"} {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)

			probe().Handler().ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusNotFound, recorder.Code, target)
		}
	})
}
