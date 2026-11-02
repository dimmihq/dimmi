package telemetry_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
request is one HTTP request a collector received.
*/
type request struct {
	path   string
	header http.Header
	body   string
}

/*
collector stands in for a telemetry backend: it accepts every request
and keeps it for the test to inspect.
*/
type collector struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []request
}

/*
newCollector starts a collector that is closed when the test ends.
*/
func newCollector(t *testing.T) *collector {
	t.Helper()
	c := &collector{}

	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		c.mu.Lock()
		c.requests = append(c.requests, request{path: r.URL.Path, header: r.Header.Clone(), body: string(body)})
		c.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(c.server.Close)

	return c
}

/*
received returns every request so far.
*/
func (c *collector) received() []request {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]request(nil), c.requests...)
}
