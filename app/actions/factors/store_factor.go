/*
|--------------------------------------------------------------------------
| Storing Factors
|--------------------------------------------------------------------------
|
| A new factor is enrolled by its driver and stored in one step, so its
| secret is sealed bound to the id it is stored under. A user has at
| most one live set of recovery codes: a new set always replaces the old
| one in the same transaction, so old codes stop working the moment new
| ones exist, and a code in flight on a sign-in either lands before the
| swap or finds the set gone.
|
*/

package factors

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NewAuthFactor is a factor just stored, with what the user sees of it
once: the setup of an authenticator or a passkey, or the codes of a
recovery set.
*/
type NewAuthFactor struct {
	Factor models.UserAuthFactor
	Setup  factors.Setup
}

/*
writer stores factors through the queries, at the time of the change,
for a user holding the factors in held.
*/
type writer struct {
	q    *models.Queries
	now  time.Time
	held []models.UserAuthFactor
}

/*
replaceRecoveryCodes deletes the user's recovery sets and stores a new
one.
*/
func (w writer) replaceRecoveryCodes(ctx context.Context, driver factors.Factor, user models.User) (
	NewAuthFactor, error,
) {
	for _, status := range enums.AuthFactorStatuses() {
		err := w.q.DeleteAuthFactorsOfType(ctx, models.DeleteAuthFactorsOfTypeParams{
			UserID: user.ID, Type: factors.RecoveryCodeType, Status: string(status),
		})
		if err != nil {
			return NewAuthFactor{}, fmt.Errorf("delete old recovery codes: %w", err)
		}
	}

	return w.store(ctx, driver, user, "")
}

/*
store enrolls a new factor of the driver's kind for the user and stores
it under the name, or the kind's default name.
*/
func (w writer) store(ctx context.Context, driver factors.Factor, user models.User, name string) (
	NewAuthFactor, error,
) {
	id := ids.NewV7At(w.now)

	enrollment, err := driver.Enroll(ctx, factors.EnrollInput{User: user, ID: id, Held: w.held})
	if err != nil {
		return NewAuthFactor{}, err
	}

	if name == "" {
		name = driver.DefaultName()
	}

	var confirmed *time.Time
	if enrollment.Status == enums.AuthFactorStatusActive {
		confirmed = &w.now
	}

	f, err := w.q.CreateAuthFactor(ctx, models.CreateAuthFactorParams{
		ID: id, UserID: user.ID, Type: driver.Type(), Name: name, Status: string(enrollment.Status),
		SecretEncrypted: enrollment.Secret, Credential: enrollment.Credential, ConfirmedAt: confirmed, Now: w.now,
	})
	if err != nil {
		return NewAuthFactor{}, fmt.Errorf("store %s factor: %w", driver.Type(), err)
	}

	return NewAuthFactor{Factor: f, Setup: enrollment.Setup}, nil
}

/*
factorEvent is the security log detail of an entry about a factor.
*/
func factorEvent(f models.UserAuthFactor) map[string]any {
	return map[string]any{"factor_id": models.PublicID(models.PrefixAuthFactor, f.ID), "type": f.Type}
}
