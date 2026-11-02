/*
|--------------------------------------------------------------------------
| Local Pacer
|--------------------------------------------------------------------------
|
| Some work must be spread out rather than counted: a crawler that may hit
| one host only every few seconds, a job that may call one account's API
| only so often. Each key gets a token bucket in process memory, so pacing
| costs no round trip and holds even while Valkey is down. Limits are per
| process, which is right when one job owns the work for a key.
|
| A bucket that has refilled completely carries no information, so it is
| dropped; the set of keys never grows past those paced recently.
|
*/

package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
minSweep is the shortest pause between two passes that drop full buckets,
so a fast rate does not turn every call into a pass over all keys.
*/
const minSweep = time.Second

/*
Local paces events per key in this process: one event every interval on
average, with bursts of up to burst events after a quiet spell.
*/
type Local struct {
	clock   clock.Clock
	every   rate.Limit
	burst   int
	sweep   time.Duration
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	swept   time.Time
}

/*
NewLocal returns a pacer allowing one event per key every interval, with
bursts of up to burst events. The clock drives waiting and refilling.
*/
func NewLocal(interval time.Duration, burst int, clk clock.Clock) (*Local, error) {
	if interval <= 0 || burst <= 0 {
		return nil, ErrInvalidLimit
	}

	return &Local{
		clock:   clk,
		every:   rate.Every(interval),
		burst:   burst,
		sweep:   max(interval*time.Duration(burst), minSweep),
		buckets: map[string]*rate.Limiter{},
		swept:   clk.Now(),
	}, nil
}

/*
Allow takes a token for the key if one is available now and reports
whether it did. Use it to skip or postpone work rather than wait.
*/
func (l *Local) Allow(key string) bool {
	now := l.clock.Now()

	return l.bucket(key, now).AllowN(now, 1)
}

/*
Wait takes a token for the key, waiting until one is available or the
context ends. A wait cut short gives its token back.
*/
func (l *Local) Wait(ctx context.Context, key string) error {
	now := l.clock.Now()
	reservation := l.bucket(key, now).ReserveN(now, 1)

	delay := reservation.DelayFrom(now)
	if delay == 0 {
		return nil
	}

	select {
	case <-l.clock.After(delay):
		return nil
	case <-ctx.Done():
		reservation.CancelAt(l.clock.Now())

		return fmt.Errorf("wait %q: %w", key, ctx.Err())
	}
}

/*
bucket returns the key's bucket, creating a full one for a new key. Now
and then it drops every bucket that is full again, which loses nothing:
a new bucket starts full too.
*/
func (l *Local) bucket(key string, now time.Time) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.swept) >= l.sweep {
		l.dropFull(now)
	}

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = rate.NewLimiter(l.every, l.burst)
		l.buckets[key] = bucket
	}

	return bucket
}

func (l *Local) dropFull(now time.Time) {
	for key, bucket := range l.buckets {
		if bucket.TokensAt(now) >= float64(l.burst) {
			delete(l.buckets, key)
		}
	}

	l.swept = now
}
