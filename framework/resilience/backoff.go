/*
|--------------------------------------------------------------------------
| Retry Backoff
|--------------------------------------------------------------------------
|
| Retrying at once hammers a provider that is already struggling, and
| many clients retrying in step make it worse. Each wait grows by a
| factor up to a ceiling, and jitter spreads clients apart. The policy
| only produces the waits; the caller sleeps on its own clock, so tests
| never wait for real.
|
*/

package resilience

import (
	"cmp"
	"iter"
	"time"

	"github.com/cenkalti/backoff/v5"
)

/*
Default backoff policy values, used for every zero field of Policy.
*/
const (
	DefaultInitial    = 500 * time.Millisecond
	DefaultMax        = time.Minute
	DefaultMultiplier = 1.5
)

/*
Policy shapes a series of waits.

Initial is the first wait, each next one is Multiplier times longer, and
none exceeds Max. Jitter spreads each wait randomly by up to that
fraction either way; zero gives exact waits. Retries caps how many waits
are produced; zero produces them until the caller stops.
*/
type Policy struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
	Jitter     float64
	Retries    int
}

/*
Backoff yields the waits of the policy, one per retry:

	for wait := range resilience.Backoff(policy) {
		if err = call(ctx); err == nil {
			break
		}
		clk.Sleep(wait)
	}
*/
func Backoff(p Policy) iter.Seq[time.Duration] {
	return func(yield func(time.Duration) bool) {
		b := &backoff.ExponentialBackOff{
			InitialInterval:     cmp.Or(p.Initial, DefaultInitial),
			MaxInterval:         cmp.Or(p.Max, DefaultMax),
			Multiplier:          cmp.Or(p.Multiplier, DefaultMultiplier),
			RandomizationFactor: p.Jitter,
		}
		b.Reset()

		for i := 0; p.Retries == 0 || i < p.Retries; i++ {
			if !yield(b.NextBackOff()) {
				return
			}
		}
	}
}
