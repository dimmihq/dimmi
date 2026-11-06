/*
|--------------------------------------------------------------------------
| Circuit Breaker
|--------------------------------------------------------------------------
|
| A connection that keeps failing stops costing real calls: after a
| run of retryable failures it opens, calls fail fast, and one probe
| slips through after the cool-down to learn whether the provider
| woke up.
|
*/

package llmretry

import (
	"sync"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Breaker is one circuit breaker per connection and model.
*/
type Breaker struct {
	mu       sync.Mutex
	clock    clock.Clock
	failures int
	max      int
	window   time.Duration
	openFor  time.Duration
	first    time.Time
	openedAt time.Time
	probing  bool
}

/*
NewBreaker builds a breaker: it opens after max failures inside the
window and stays open for openFor.
*/
func NewBreaker(clk clock.Clock, max int, window, openFor time.Duration) *Breaker {
	return &Breaker{clock: clk, max: max, window: window, openFor: openFor}
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

	if b.clock.Now().Before(b.openedAt.Add(b.openFor)) {
		return &contracts.LLMError{Kind: contracts.LLMErrorOverloaded, Message: "circuit open"}
	}

	if b.probing {
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
Failure counts a retryable failure and opens at the limit.
*/
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	if b.failures == 0 || now.Sub(b.first) > b.window {
		b.failures = 0
		b.first = now
	}

	b.failures++
	if b.failures >= b.max {
		b.openedAt = now
		b.probing = false
	}
}
