package httpclient_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/drivertest"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

/*
fakeBase is where the fake serves contract routes. It is a public name,
so only the stubs decide what it answers.
*/
const fakeBase = "https://api.example.test"

/*
serve starts a loopback server for the handler and returns it with a
client allowed to reach it by host:port.
*/
func serve(t *testing.T, handler http.Handler) (*httptest.Server, *httpclient.Client) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, newClient(t, httpclient.Config{AllowHosts: []string{hostOf(t, server)}})
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

func hostOf(t *testing.T, server *httptest.Server) string {
	t.Helper()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	return u.Host
}

func realClient(t *testing.T, routes []drivertest.Route) (*httpclient.Client, string) {
	t.Helper()

	mux := http.NewServeMux()
	for _, route := range routes {
		mux.HandleFunc(route.Method+" "+route.Path, func(w http.ResponseWriter, _ *http.Request) {
			maps.Copy(w.Header(), route.Response.Header)

			w.WriteHeader(route.Response.Status)
			_, _ = w.Write(route.Response.Body)
		})
	}

	server, client := serve(t, mux)

	return client, server.URL
}

func fakeClient(t *testing.T, routes []drivertest.Route) (*httpclient.Client, string) {
	t.Helper()

	client := newClient(t, httpclient.Config{})
	fake := httpclienttest.New(client).PreventStrayRequests()

	for _, route := range routes {
		fake.Stub(route.Method+" "+fakeBase+route.Path, route.Response)
	}

	return client, fakeBase
}
