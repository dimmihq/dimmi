package httpclienttest

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

type cannedTransport struct{ calls int }

func (c *cannedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls++

	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"page":` + req.URL.Query().Get("page") + `}`)), Request: req,
	}, nil
}

func TestRecordAndReplay(t *testing.T) {
	t.Parallel()

	t.Run("records every exchange and replays it in order", func(t *testing.T) {
		t.Parallel()

		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)

		rec := Record(client)
		canned := &cannedTransport{}
		rec.next = canned

		for _, page := range []string{"1", "2"} {
			resp, err := client.New().Get(t.Context(), "https://api.example.com/items?page="+page)
			require.NoError(t, err)
			assert.JSONEq(t, `{"page":`+page+`}`, string(resp.Body))
		}

		exchanges := rec.Exchanges()
		require.Len(t, exchanges, 2)
		assert.Equal(t, "application/json", exchanges[0].ContentType)

		replayed, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)
		New(replayed).PreventStrayRequests().Replay(exchanges)

		resp, err := replayed.New().Get(t.Context(), "https://api.example.com/items?page=2")
		require.NoError(t, err)
		assert.JSONEq(t, `{"page":2}`, string(resp.Body))
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		_, err = replayed.New().Get(t.Context(), "https://api.example.com/items?page=3")
		require.ErrorIs(t, err, ErrStrayRequest)
		assert.Equal(t, 2, canned.calls)
	})
}

type gzipTransport struct{ body []byte }

func (g gzipTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Encoding": {"gzip"}},
		Body: io.NopCloser(bytes.NewReader(g.body)), Request: req,
	}, nil
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestRecordedBodies(t *testing.T) {
	t.Parallel()

	t.Run("a gzipped answer is recorded plainly", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte(`{"ok":true}`))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		assert.JSONEq(t, `{"ok":true}`, decoded("gzip", buf.Bytes()))
	})

	t.Run("a broken gzip answer is recorded as it came", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "not gzip", decoded("gzip", []byte("not gzip")))

		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte(strings.Repeat("x", 100)))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		cut := buf.Bytes()[:buf.Len()-8]
		assert.Equal(t, string(cut), decoded("gzip", cut))
	})

	t.Run("the recorder passes the answer through and keeps it", func(t *testing.T) {
		t.Parallel()

		rec := &Recorder{next: gzipTransport{body: []byte("raw")}}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/x", http.NoBody)
		require.NoError(t, err)

		resp, err := rec.RoundTrip(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, "raw", string(body))
		require.Len(t, rec.Exchanges(), 1)
	})

	t.Run("a failed request is not recorded", func(t *testing.T) {
		t.Parallel()

		rec := &Recorder{next: failingTransport{}}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com/x", http.NoBody)
		require.NoError(t, err)

		resp, err := rec.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		require.Error(t, err)
		assert.Empty(t, rec.Exchanges())
	})
}
