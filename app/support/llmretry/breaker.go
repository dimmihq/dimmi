/*
|--------------------------------------------------------------------------
| Circuit Breaker
|--------------------------------------------------------------------------
|
| A connection that keeps failing stops costing real calls: after a
| run of retryable failures it opens, calls fail fast, and one probe
| slips through after the cool-down to learn whether the provider
| woke up. A failed probe opens it for another cool-down; a probe the
| caller abandons frees the slot for the next one, so the breaker can
| never stay shut for good.
|
*/

package llmretry

import (
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/config"
)

/*
Breaker is one circuit breaker per connection and model.
*/
type Breaker struct {
	mu       sync.Mutex
	clock    clock.Clock
	failures int
	limit    int
	window   time.Duration
	openFor  time.Duration
	first    time.Time
	openedAt time.Time
	probing  bool
}

/*
NewBreaker builds a breaker: it opens after limit failures inside the
window and stays open for openFor.
*/
func NewBreaker(clk clock.Clock, limit int, window, openFor time.Duration) *Breaker {
	return &Breaker{clock: clk, limit: max(limit, 1), window: window, openFor: openFor}
}

/*
Allow reports whether a call may proceed: open breakers fail fast
with overloaded, except one trial call after the cool-down.
*/
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openedAt.IsZero() {
		return nil
	}

	if b.probing || b.clock.Now().Before(b.openedAt.Add(b.openFor)) {
		return &contracts.LLMError{Kind: contracts.LLMErrorOverloaded, Message: "circuit open"}
	}

	b.probing = true
	return nil
}

/*
Success closes the breaker.
*/
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openedAt = time.Time{}
	b.probing = false
}

/*
Release frees the trial slot of a probe that ended without a verdict,
such as one the caller canceled, leaving the breaker as it was.
*/
func (b *Breaker) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
}

/*
Failure counts a retryable failure and opens at the limit; a failed
probe reopens the breaker for a fresh cool-down at once.
*/
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	if b.probing {
		b.openedAt = now
		b.probing = false
		return
	}

	if b.failures == 0 || now.Sub(b.first) > b.window {
		b.failures = 0
		b.first = now
	}

	b.failures++
	if b.failures >= b.limit {
		b.openedAt = now
	}
}

/*
Breakers hands out one breaker per name, built from the configured
limits, and keeps them for the process's life so a rebuilt driver
inherits the state of the one it replaced.
*/
type Breakers struct {
	mu    sync.Mutex
	clock clock.Clock
	cfg   config.AIBreakerConfig
	all   map[string]*Breaker
}

/*
NewBreakers builds the registry.
*/
func NewBreakers(clk clock.Clock, cfg config.AIBreakerConfig) *Breakers {
	return &Breakers{clock: clk, cfg: cfg, all: map[string]*Breaker{}}
}

/*
For returns the breaker for a name, creating it on first use.
*/
func (r *Breakers) For(name string) *Breaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok := r.all[name]; ok {
		return b
	}
	b := NewBreaker(r.clock, r.cfg.Failures, r.cfg.Window, r.cfg.Open)
	r.all[name] = b
	return b
}
