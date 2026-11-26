package connect

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/cli/dimmi"
)

func TestPageIsSecretAndRefusesOtherOrigins(t *testing.T) {
	ctx := t.Context()

	var saved string

	s := &Server{
		DimmiURL: "https://dimmi.example", SaveToken: func(dimmi.Token) error { return nil }, DimmiDone: func() bool { return false },
		Source: Setup{
			Tool:  "Intercom",
			Check: func(context.Context, string, string) (string, error) { return "Acme", nil },
			Save:  func(_, token string) error { saved = token; return nil },
			Saved: func() (string, bool) { return "", saved != "" },
		},
	}

	addr, stop, err := s.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = stop() }()

	if !strings.HasPrefix(addr, "http://127.0.0.1:") {
		t.Fatalf("listens at %s, not loopback", addr)
	}

	u, _ := url.Parse(addr)
	root := u.Scheme + "://" + u.Host

	code, err := send(t, request{method: http.MethodGet, target: root + "/"})
	if err != nil || code != http.StatusNotFound {
		t.Fatalf("the page answered without its secret path: %v %v", code, err)
	}

	code, err = postSource(t, addr, "abc", map[string]string{"Origin": "https://evil.example"})
	if err != nil || code != http.StatusForbidden || saved != "" {
		t.Fatalf("a post from another site was taken: %v %v", code, err)
	}

	if _, err := postSource(t, addr, "abc", map[string]string{"Origin": root}); err != nil || saved != "abc" {
		t.Fatalf("the page's own post was refused: %v saved %q", err, saved)
	}

	saved = ""
	sameSite := map[string]string{"Origin": "null", "Sec-Fetch-Site": "same-origin"}

	if _, err := postSource(t, addr, "def", sameSite); err != nil || saved != "def" {
		t.Fatalf("a same-origin post with a null origin was refused: %v saved %q", err, saved)
	}

	saved = ""
	crossSite := map[string]string{"Origin": "null", "Sec-Fetch-Site": "cross-site"}

	code, err = postSource(t, addr, "ghi", crossSite)
	if err != nil || code != http.StatusForbidden || saved != "" {
		t.Fatalf("a cross-site post with a null origin was taken: %v saved %q", err, saved)
	}
}

/*
postSource posts a token to the page's source form with the headers.
*/
func postSource(t *testing.T, addr, token string, headers map[string]string) (int, error) {
	t.Helper()

	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	maps.Copy(h, headers)

	return send(t, request{
		method: http.MethodPost, target: addr + "source", body: "region=us&token=" + token, headers: h,
	})
}

/*
request is one request a test sends to the page.
*/
type request struct {
	method  string
	target  string
	body    string
	headers map[string]string
}

/*
send makes a request and returns its status, closing the body.
*/
func send(t *testing.T, r request) (int, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), r.method, r.target, strings.NewReader(r.body))
	if err != nil {
		return 0, err
	}

	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	_ = res.Body.Close()

	return res.StatusCode, nil
}
