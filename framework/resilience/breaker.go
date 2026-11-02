/*
|--------------------------------------------------------------------------
| Circuit Breaker
|--------------------------------------------------------------------------
|
| A provider that keeps failing should stop receiving calls for a while,
| so requests fail fast instead of queuing behind timeouts, and the
| provider gets room to recover. After a cooldown a few probe calls go
| through; their outcome closes the breaker again or keeps it open.
|
| A caller giving up is not the provider failing, so a canceled context
| never counts against it.
|
*/

package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
)

/*
ErrOpen is returned without calling the function while the breaker is
open, or while it is half open and its probe calls are already in
flight.
*/
var ErrOpen = errors.New("resilience: circuit open")

/*
Default breaker settings, used for every zero field of Settings.
*/
const (
	DefaultFailures = 5
	DefaultCooldown = 30 * time.Second
	DefaultProbes   = 1
)

/*
Settings tune a breaker.

Failures is how many failures in a row open it. Cooldown is how long it
stays open before letting probes through. Probes is how many calls may
run while half open. Interval clears the failure count of a closed
breaker periodically; zero never clears it. IsFailure decides which
errors count as failures; nil counts every error.
*/
type Settings struct {
	Failures  uint32
	Cooldown  time.Duration
	Probes    uint32
	Interval  time.Duration
	IsFailure func(err error) bool
}

/*
Breaker guards calls to one provider. It is safe for concurrent use.
*/
type Breaker struct {
	cb *gobreaker.CircuitBreaker[struct{}]
}

/*
NewBreaker returns a closed breaker. The name identifies the provider in
errors.
*/
func NewBreaker(name string, s Settings) *Breaker {
	if s.Failures == 0 {
		s.Failures = DefaultFailures
	}

	if s.Cooldown == 0 {
		s.Cooldown = DefaultCooldown
	}

	if s.Probes == 0 {
		s.Probes = DefaultProbes
	}

	failures := s.Failures
	isFailure := s.IsFailure

	return &Breaker{cb: gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        name,
		MaxRequests: s.Probes,
		Interval:    s.Interval,
		Timeout:     s.Cooldown,
		ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.ConsecutiveFailures >= failures },
		IsSuccessful: func(err error) bool {
			return err == nil || (isFailure != nil && !isFailure(err))
		},
		IsExcluded: func(err error) bool { return errors.Is(err, context.Canceled) },
	})}
}

/*
Execute calls the function unless the breaker is open and records the
outcome. The function's own error is returned unchanged.
*/
func (b *Breaker) Execute(fn func() error) error {
	_, err := b.cb.Execute(func() (struct{}, error) { return struct{}{}, fn() })
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("%s: %w", b.cb.Name(), ErrOpen)
	}

	return err
}
