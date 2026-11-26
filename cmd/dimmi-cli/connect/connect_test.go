package connect

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/dimmi"
)

func TestPageIsSecretAndRefusesOtherOrigins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	defer stop()

	if !strings.HasPrefix(addr, "http://127.0.0.1:") {
		t.Fatalf("listens at %s, not loopback", addr)
	}

	u, _ := url.Parse(addr)
	root := u.Scheme + "://" + u.Host

	if res, err := http.Get(root + "/"); err != nil || res.StatusCode != http.StatusNotFound {
		t.Fatalf("the page answered without its secret path: %v %v", res.StatusCode, err)
	}

	req, _ := http.NewRequest(http.MethodPost, addr+"source", strings.NewReader("region=us&token=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusForbidden || saved != "" {
		t.Fatalf("a post from another site was taken: %v %v", res.StatusCode, err)
	}

	req, _ = http.NewRequest(http.MethodPost, addr+"source", strings.NewReader("region=us&token=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", root)

	if _, err := http.DefaultClient.Do(req); err != nil || saved != "abc" {
		t.Fatalf("the page's own post was refused: %v saved %q", err, saved)
	}
}
