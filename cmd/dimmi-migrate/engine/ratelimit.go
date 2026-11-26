package engine

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

/*
Limiter is an http.RoundTripper that keeps a client inside an API's
rate limit. It reads the X-RateLimit-Limit, -Remaining and -Reset
headers every answer carries and, once less than Reserve of the limit
is left, waits for the window to reset. It also spaces requests to at
most Rate a second, and retries a 429 or 503 after Retry-After or the
reset, and a dropped connection with backoff.

Share stays below what the API allows, so the customer's own
integrations keep working while a migration runs: with Share 0.3 the
limiter acts as if only 30% of the window were there.
*/
type Limiter struct {
	Next       http.RoundTripper
	Share      float64
	Rate       float64
	MaxRetries int
	MaxWait    time.Duration
	Clock      func() time.Time
	Sleep      func(ctx context.Context, d time.Duration) error

	mu        sync.Mutex
	limit     int
	remaining int
	reset     time.Time
	next      time.Time
}

/*
NewLimiter returns a limiter over the transport.
*/
func NewLimiter(next http.RoundTripper, share, rate float64) *Limiter {
	if next == nil {
		next = http.DefaultTransport
	}

	return &Limiter{Next: next, Share: share, Rate: rate, MaxRetries: 8, MaxWait: 2 * time.Minute}
}

func (l *Limiter) now() time.Time {
	if l.Clock != nil {
		return l.Clock()
	}

	return time.Now()
}

func (l *Limiter) sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	if l.Sleep != nil {
		return l.Sleep(ctx, d)
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

/*
RoundTrip sends the request once the limit allows it, retrying what the
API asks to be retried.
*/
func (l *Limiter) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := rewindable(req)
	if err != nil {
		return nil, err
	}

	ctx := req.Context()

	for attempt := 0; ; attempt++ {
		if err := l.sleep(ctx, l.wait()); err != nil {
			return nil, err
		}

		try := req.Clone(ctx)

		switch {
		case body != nil:
			try.Body = io.NopCloser(bytes.NewReader(body))
			try.ContentLength = int64(len(body))
		case attempt > 0 && req.GetBody != nil:
			if try.Body, err = req.GetBody(); err != nil {
				return nil, err
			}
		case attempt > 0 && req.Body != nil && req.Body != http.NoBody:
			return nil, errors.New("request body can't be sent again")
		}

		res, err := l.Next.RoundTrip(try)
		if err != nil {
			if attempt >= l.MaxRetries || ctx.Err() != nil {
				return nil, err
			}

			if err := l.sleep(ctx, backoff(attempt)); err != nil {
				return nil, err
			}

			continue
		}

		l.observe(res.Header)

		if (res.StatusCode != http.StatusTooManyRequests && res.StatusCode != http.StatusServiceUnavailable) ||
			attempt >= l.MaxRetries {
			return res, nil
		}

		wait := l.retryAfter(res.Header, attempt)
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
		_ = res.Body.Close()

		if err := l.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

/*
rewindable reads a request's body so it can be sent again. Bodies the
engine sends are batches of at most a few megabytes; a file upload is
read from disk and can be large, so a request with GetBody keeps its own.
*/
func rewindable(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	if req.GetBody != nil && req.ContentLength > 8<<20 {
		return nil, nil
	}

	defer req.Body.Close()

	return io.ReadAll(req.Body)
}

/*
wait returns how long the next request must wait.
*/
func (l *Limiter) wait() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	var d time.Duration

	if l.limit > 0 && !l.reset.IsZero() && now.Before(l.reset) {
		keep := int(float64(l.limit) * (1 - l.Share))
		if l.remaining <= keep {
			d = l.reset.Sub(now)
		}
	}

	if l.Rate > 0 {
		gap := time.Duration(float64(time.Second) / l.Rate)
		start := now.Add(d)

		if start.Before(l.next) {
			start = l.next
		}

		l.next = start.Add(gap)
		d = start.Sub(now)
	}

	if l.limit > 0 && l.remaining > 0 {
		l.remaining--
	}

	if l.MaxWait > 0 && d > l.MaxWait {
		d = l.MaxWait
	}

	return d
}

/*
observe records the limit headers of an answer.
*/
func (l *Limiter) observe(h http.Header) {
	limit, okLimit := headerInt(h, "X-RateLimit-Limit")
	remaining, okRemaining := headerInt(h, "X-RateLimit-Remaining")
	reset, okReset := headerInt(h, "X-RateLimit-Reset")

	if !okLimit && !okRemaining {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if okLimit {
		l.limit = limit
	}

	if okRemaining {
		l.remaining = remaining
	}

	if okReset {
		l.reset = resetTime(l.now(), int64(reset))
	}
}

/*
resetTime reads a reset header: a Unix time when it is large, seconds
from now when it is small.
*/
func resetTime(now time.Time, v int64) time.Time {
	if v > 1_000_000_000 {
		return time.Unix(v, 0)
	}

	return now.Add(time.Duration(v) * time.Second)
}

/*
retryAfter returns how long to wait before retrying a refused request.
*/
func (l *Limiter) retryAfter(h http.Header, attempt int) time.Duration {
	var d time.Duration

	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			d = time.Duration(secs) * time.Second
		} else if t, err := http.ParseTime(v); err == nil {
			d = t.Sub(l.now())
		}
	}

	if d <= 0 {
		if reset, ok := headerInt(h, "X-RateLimit-Reset"); ok {
			d = resetTime(l.now(), int64(reset)).Sub(l.now())
		}
	}

	if d <= 0 {
		d = backoff(attempt)
	}

	if l.MaxWait > 0 && d > l.MaxWait {
		d = l.MaxWait
	}

	return d
}

func headerInt(h http.Header, name string) (int, bool) {
	v := h.Get(name)
	if v == "" {
		return 0, false
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		f, ferr := strconv.ParseFloat(v, 64)
		if ferr != nil {
			return 0, false
		}

		n = int(f)
	}

	return n, true
}

/*
backoff is the wait before retry n: 1s, 2s, 4s… up to 30s, with jitter.
*/
func backoff(attempt int) time.Duration {
	d := time.Second << min(attempt, 5)
	if d > 30*time.Second {
		d = 30 * time.Second
	}

	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

/*
ErrStop ends a read early without failing it.
*/
var ErrStop = errors.New("stop")
