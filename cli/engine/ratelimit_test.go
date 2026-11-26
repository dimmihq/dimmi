package engine

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

type scripted struct {
	answers []*http.Response
	bodies  []string
}

func (s *scripted) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		s.bodies = append(s.bodies, string(raw))
	}

	res := s.answers[0]
	s.answers = s.answers[1:]

	return res, nil
}

func answer(code int, headers map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}

	return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(""))}
}

type sleeps struct{ waits []time.Duration }

func (s *sleeps) sleep(_ context.Context, d time.Duration) error {
	s.waits = append(s.waits, d)

	return nil
}

func TestLimiterRetries429AfterRetryAfterWithTheSameBody(t *testing.T) {
	next := &scripted{answers: []*http.Response{
		answer(429, map[string]string{"Retry-After": "7"}),
		answer(200, nil),
	}}
	sl := &sleeps{}
	l := NewLimiter(next, 1, 0)
	l.Sleep = sl.sleep

	req, _ := http.NewRequest(http.MethodPost, "https://api.example.com/x", strings.NewReader(`{"a":1}`))

	res, err := l.RoundTrip(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("%v %v", res, err)
	}

	if len(next.bodies) != 2 || next.bodies[1] != `{"a":1}` {
		t.Fatalf("bodies %q", next.bodies)
	}

	if !containsWait(sl.waits, 7*time.Second) {
		t.Fatalf("waits %v", sl.waits)
	}
}

func TestLimiterWaitsForTheResetOnceItsShareIsSpent(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	next := &scripted{answers: []*http.Response{
		answer(200, map[string]string{"X-RateLimit-Limit": "100", "X-RateLimit-Remaining": "70", "X-RateLimit-Reset": "1800000009"}),
		answer(200, nil),
	}}
	sl := &sleeps{}
	l := NewLimiter(next, 0.3, 0)
	l.Sleep = sl.sleep
	l.Clock = func() time.Time { return now }

	for range 2 {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", http.NoBody)
		if _, err := l.RoundTrip(req); err != nil {
			t.Fatal(err)
		}
	}

	if !containsWait(sl.waits, 9*time.Second) {
		t.Fatalf("waits %v: with 30%% of 100 used it must wait for the reset", sl.waits)
	}
}

func TestLimiterRetriesOn429UsingTheReset(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	next := &scripted{answers: []*http.Response{
		answer(429, map[string]string{"X-RateLimit-Reset": "1800000004"}),
		answer(200, nil),
	}}
	sl := &sleeps{}
	l := NewLimiter(next, 1, 0)
	l.Sleep = sl.sleep
	l.Clock = func() time.Time { return now }

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", http.NoBody)
	if _, err := l.RoundTrip(req); err != nil {
		t.Fatal(err)
	}

	if !containsWait(sl.waits, 4*time.Second) {
		t.Fatalf("waits %v", sl.waits)
	}
}

func containsWait(waits []time.Duration, d time.Duration) bool {
	return slices.Contains(waits, d)
}
