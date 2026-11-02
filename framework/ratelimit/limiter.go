/*
|--------------------------------------------------------------------------
| Rate Limiter
|--------------------------------------------------------------------------
|
| A limiter allows a number of attempts per key in a fixed window, counted
| in Valkey so every process sees the same count. Both keys of a counter
| share one hash tag, so the limiter behaves the same on a single server
| and on a cluster.
|
| The limiter never decides what a Valkey outage means. Guards against
| abuse must refuse while counts cannot be checked, other limits should
| let the work through, so every failure to reach Valkey comes back as
| ErrUnavailable and the caller picks.
|
*/

package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislimiter"

	"github.com/dimmihq/dimmi/framework/redis"
)

/*
Errors a limiter returns.

ErrInvalidLimit means the maximum or the decay is not positive.
ErrUnavailable wraps every failure to reach Valkey: deny the attempt to
fail closed, or proceed and log it to fail open.
*/
var (
	ErrInvalidLimit = errors.New("ratelimit: max attempts and decay must be positive")
	ErrUnavailable  = errors.New("ratelimit: valkey unavailable")
)

/*
Options configure a limiter. The keyspace keeps its counters apart from
every other service on the same server.
*/
type Options struct {
	Keyspace string `default:"rl" koanf:"keyspace"`
}

/*
Result is the state of a key after a call.

Allowed reports whether the attempt may go ahead. Limit is the maximum
the call was made with and Remaining how many attempts are left in the
current window. RetryAfter is how long until the window resets, and is
zero while attempts remain.
*/
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

/*
Limiter counts attempts per key on the shared Valkey client.
*/
type Limiter struct {
	limiter rueidislimiter.RateLimiterClient
}

/*
New returns the limiter for a keyspace on the given client.

The library asks for a default limit, but every call passes its own, so
the default is never used.
*/
func New(client *redis.Client, opts Options) (*Limiter, error) {
	limiter, err := rueidislimiter.NewRateLimiter(rueidislimiter.RateLimiterOption{
		ClientBuilder: func(rueidis.ClientOption) (rueidis.Client, error) { return client.Rueidis(), nil },
		KeyPrefix:     strings.TrimSuffix(client.Prefix(opts.Keyspace), ":"),
		Limit:         1,
		Window:        time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open rate limiter: %w", err)
	}

	return &Limiter{limiter: limiter}, nil
}

/*
Attempt counts one attempt at the key if fewer than maxAttempts were made
in the current window. A refused attempt is not counted, so a caller that
keeps retrying is let through as soon as the window resets.
*/
func (l *Limiter) Attempt(ctx context.Context, key string, maxAttempts int, decay time.Duration) (Result, error) {
	limit, err := window(maxAttempts, decay)
	if err != nil {
		return Result{}, err
	}

	res, err := l.limiter.AllowAtMost(ctx, key, 1, limit)
	if err != nil {
		return Result{}, fmt.Errorf("attempt %q: %w: %w", key, ErrUnavailable, err)
	}

	return result(maxAttempts, res), nil
}

/*
Hit counts one attempt at the key even past the limit, for counters where
every event must count, such as failed sign-ins that lead to a lockout.
*/
func (l *Limiter) Hit(ctx context.Context, key string, maxAttempts int, decay time.Duration) (Result, error) {
	limit, err := window(maxAttempts, decay)
	if err != nil {
		return Result{}, err
	}

	res, err := l.limiter.Allow(ctx, key, limit)
	if err != nil {
		return Result{}, fmt.Errorf("hit %q: %w: %w", key, ErrUnavailable, err)
	}

	return result(maxAttempts, res), nil
}

/*
Remaining returns how many attempts are left at the key in the current
window, without counting one.
*/
func (l *Limiter) Remaining(ctx context.Context, key string, maxAttempts int, decay time.Duration) (int, error) {
	res, err := l.check(ctx, key, maxAttempts, decay)
	if err != nil {
		return 0, fmt.Errorf("remaining %q: %w", key, err)
	}

	return res.Remaining, nil
}

/*
AvailableIn returns how long until the key accepts attempts again, or
zero when it accepts them now.
*/
func (l *Limiter) AvailableIn(ctx context.Context, key string, maxAttempts int, decay time.Duration) (
	time.Duration, error,
) {
	res, err := l.check(ctx, key, maxAttempts, decay)
	if err != nil {
		return 0, fmt.Errorf("available in %q: %w", key, err)
	}

	return res.RetryAfter, nil
}

/*
Clear resets the key's count, as after a successful sign-in.
*/
func (l *Limiter) Clear(ctx context.Context, key string) error {
	if err := l.limiter.Reset(ctx, key); err != nil {
		return fmt.Errorf("clear %q: %w: %w", key, ErrUnavailable, err)
	}

	return nil
}

func (l *Limiter) check(ctx context.Context, key string, maxAttempts int, decay time.Duration) (Result, error) {
	limit, err := window(maxAttempts, decay)
	if err != nil {
		return Result{}, err
	}

	res, err := l.limiter.Check(ctx, key, limit)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	return result(maxAttempts, res), nil
}

func window(maxAttempts int, decay time.Duration) (rueidislimiter.RateLimitOption, error) {
	if maxAttempts <= 0 || decay <= 0 {
		return rueidislimiter.RateLimitOption{}, ErrInvalidLimit
	}

	return rueidislimiter.WithCustomRateLimit(maxAttempts, decay), nil
}

/*
result converts the library's answer. The library reports a negative
retry delay for an allowed attempt; callers get zero.
*/
func result(maxAttempts int, res rueidislimiter.Result) Result {
	return Result{
		Allowed:    res.Allowed,
		Limit:      maxAttempts,
		Remaining:  int(res.Remaining),
		RetryAfter: max(res.RetryAfter, 0),
	}
}
