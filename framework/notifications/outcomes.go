/*
|--------------------------------------------------------------------------
| Delivery Outcomes
|--------------------------------------------------------------------------
|
| Every delivery ends one of three ways: it went out, it failed, or the
| notification declined it. Observers hear each outcome as it happens, so
| the application can keep a log that answers "why did I not get that?"
| without any channel knowing about it.
|
| A notification that declines a delivery may say why by returning a
| skip from ShouldSend: the delivery is then skipped with that reason
| and nothing is retried.
|
| A send to one recipient can carry its delivery id in the context, so
| what the application records before sending, such as a channel the
| recipient turned off, shares the id of what is sent.
|
*/

package notifications

import (
	"context"
	"errors"
)

/*
The ways a delivery ends.
*/
const (
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

/*
Outcome is how one delivery ended: sent, failed with its error, or
skipped with the reason the notification gave, "" when it gave none.
*/
type Outcome struct {
	Delivery     Delivery
	Notification Notification
	Status       string
	Reason       string
	Err          error
}

/*
Observer hears every outcome. It runs on the sender's goroutine, so it
must be quick and must not fail the delivery.
*/
type Observer func(ctx context.Context, o Outcome)

/*
SkipError is a delivery the notification declined, with why.
*/
type SkipError struct {
	Reason string
}

/*
Error names the reason.
*/
func (e *SkipError) Error() string { return "notifications: skipped: " + e.Reason }

/*
Skip declines a delivery from ShouldSend with a reason.
*/
func Skip(reason string) error { return &SkipError{Reason: reason} }

/*
Observe adds an observer of every delivery's outcome. Add observers at
boot.
*/
func (m *Manager) Observe(fn Observer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.observers = append(m.observers, fn)
}

/*
report tells every observer how a delivery ended.
*/
func (m *Manager) report(ctx context.Context, o Outcome) {
	m.mu.RLock()
	observers := m.observers
	m.mu.RUnlock()

	for _, fn := range observers {
		fn(ctx, o)
	}
}

/*
skipReason reads the reason of a skip, reporting false for any other
error.
*/
func skipReason(err error) (string, bool) {
	if skip, ok := errors.AsType[*SkipError](err); ok {
		return skip.Reason, true
	}

	return "", false
}

type deliveryIDKey struct{}

/*
WithDeliveryID makes the next send to one recipient use the id, so
what the application records about the same notification shares it.
*/
func WithDeliveryID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, deliveryIDKey{}, id)
}

/*
DeliveryID returns the id the context carries, "" when none.
*/
func DeliveryID(ctx context.Context) string {
	id, _ := ctx.Value(deliveryIDKey{}).(string)

	return id
}
