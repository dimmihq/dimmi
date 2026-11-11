/*
|--------------------------------------------------------------------------
| Recorded Exchanges
|--------------------------------------------------------------------------
|
| A test against a third-party API is only as good as its stubs, and the
| best stubs are the API's own answers. Record sends a client's requests
| to the network once and keeps every exchange; Replay answers the same
| requests from those exchanges forever after, each address with its
| answers in the order they came, so paging and retries replay too.
| Recording reaches the real network, so it is for a developer's run
| against dummy data, never for CI.
|
*/

package httpclienttest

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"regexp"
	"sync"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Exchange is one request and the answer it got.
*/
type Exchange struct {
	Method      string `json:"method"`
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body"`
}

/*
Recorder passes a client's requests to the network and keeps every
exchange.
*/
type Recorder struct {
	next http.RoundTripper
	mu   sync.Mutex
	log  []Exchange
}

/*
Record sends the client's requests to the network from now on and
records them.
*/
func Record(client *httpclient.Client) *Recorder {
	r := &Recorder{next: http.DefaultTransport}
	client.ReplaceTransport(r)

	return r
}

/*
RoundTrip sends the request and keeps the answer.
*/
func (r *Recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if err != nil {
		return nil, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))

	r.mu.Lock()
	defer r.mu.Unlock()

	r.log = append(r.log, Exchange{
		Method: req.Method, URL: req.URL.String(), Status: resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"), Body: decoded(resp.Header.Get("Content-Encoding"), body),
	})

	return resp, nil
}

/*
decoded is the body as the client reads it, gzip undone, so a replay
answers plainly.
*/
func decoded(encoding string, body []byte) string {
	if encoding != "gzip" {
		return string(body)
	}

	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return string(body)
	}

	plain, err := io.ReadAll(zr)
	if err != nil {
		return string(body)
	}

	return string(plain)
}

/*
Exchanges returns what was recorded so far, in order.
*/
func (r *Recorder) Exchanges() []Exchange {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]Exchange(nil), r.log...)
}

/*
Replay answers each recorded method and address with its recorded
answers, in order, the last one repeating.
*/
func (f *Fake) Replay(exchanges []Exchange) *Fake {
	grouped := map[string][]httpclient.Response{}

	var order []string

	for _, e := range exchanges {
		key := e.Method + " " + e.URL
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}

		resp := httpclient.Response{Status: e.Status, Body: []byte(e.Body), Header: http.Header{}}
		if e.ContentType != "" {
			resp.Header.Set("Content-Type", e.ContentType)
		}

		grouped[key] = append(grouped[key], resp)
	}

	for _, key := range order {
		method, address, _ := bytes.Cut([]byte(key), []byte(" "))
		f.stubExact(string(method), string(address), grouped[key])
	}

	return f
}

/*
stubExact answers one exact method and address with the responses in
order.
*/
func (f *Fake) stubExact(method, address string, resps []httpclient.Response) {
	glob := regexp.MustCompile("^" + regexp.QuoteMeta(address) + "$")

	var mu sync.Mutex

	next := 0

	f.mock.RegisterRegexpResponder(method, glob, func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		resp := resps[min(next, len(resps)-1)]
		next++
		mu.Unlock()

		return responder(resp)(req)
	})
}
