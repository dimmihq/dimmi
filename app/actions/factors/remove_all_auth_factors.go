/*
|--------------------------------------------------------------------------
| Remove All Auth Factors
|--------------------------------------------------------------------------
|
| "Turn two-factor authentication off" in one step: every factor goes,
| unfinished ones included. The same action serves an operator who turns
| it off from the command line for a user locked out of their account.
| An account without two-factor authentication only loses its unfinished
| enrollments, and nobody is mailed about it.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RemoveAllAuthFactorsInput is the user, the client and the first-party
app grant to keep, ids.Nil() for none.
*/
type RemoveAllAuthFactorsInput struct {
	UserID    ids.UUID
	Client    appauth.Client
	KeepGrant ids.UUID
}

/*
RemoveAllAuthFactors removes every factor of the user.

Rules:
  - With two-factor authentication on, it is turned off through
    [DisableTwoFactor].
  - Otherwise only unfinished factors are deleted, if any.

It reports whether two-factor authentication was on.

Errors: those of [DisableTwoFactor], or a wrapped error.
*/
type RemoveAllAuthFactors struct {
	DB      *database.DB
	Disable *DisableTwoFactor
}

/*
Handle removes the factors.
*/
func (a *RemoveAllAuthFactors) Handle(ctx context.Context, in RemoveAllAuthFactorsInput) (bool, error) {
	var wasOn bool

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, in.UserID)
		if err != nil {
			return fmt.Errorf("lock user to remove factors: %w", err)
		}

		wasOn = user.TwoFactorEnabled()
		if wasOn {
			return a.Disable.Handle(ctx, tx, DisableTwoFactorInput{
				User: user, Client: in.Client, KeepGrant: in.KeepGrant,
			})
		}

		if _, err := q.DeleteUserAuthFactors(ctx, user.ID); err != nil {
			return fmt.Errorf("delete unfinished factors: %w", err)
		}

		return nil
	})

	return wasOn, err
}
