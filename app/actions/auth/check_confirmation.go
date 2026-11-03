/*
|--------------------------------------------------------------------------
| Check Confirmation
|--------------------------------------------------------------------------
|
| Clients ask whether the user confirmed who they are recently enough
| for a sensitive step, so they can prompt before the step is refused
| rather than after.
|
*/

package auth

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
CheckConfirmation returns the caller's confirmation while it holds.

Rules:
  - A confirmation holds for TTL after the last full sign-in or
    confirmation.

Errors: [exceptions.ErrNotFound] when none holds.
*/
type CheckConfirmation struct {
	Clock clock.Clock
	TTL   time.Duration
}

/*
Handle returns the confirmation.
*/
func (a *CheckConfirmation) Handle(_ context.Context, p *appauth.Principal) (Confirmation, error) {
	until := p.ConfirmedAt().Add(a.TTL)
	if p.ConfirmedAt().IsZero() || !until.After(a.Clock.Now()) {
		return Confirmation{}, exceptions.ErrNotFound
	}

	return Confirmation{Method: p.SignIn.ConfirmedMethod, Until: until}, nil
}
