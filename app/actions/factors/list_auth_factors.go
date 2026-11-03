/*
|--------------------------------------------------------------------------
| List Auth Factors
|--------------------------------------------------------------------------
|
| The factors a user sees in their settings: active ones, kinds the
| install no longer enables included so they can still be removed, and
| enrollments still within the time they may be finished in.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
ListAuthFactors lists a user's own factors, preferred first.

Errors: a wrapped error.
*/
type ListAuthFactors struct {
	DB      *database.DB
	Clock   clock.Clock
	Factors *factors.Registry
	Config  config.FactorsConfig
}

/*
Handle returns the user's visible factors.
*/
func (a *ListAuthFactors) Handle(ctx context.Context, userID ids.UUID) ([]models.UserAuthFactor, error) {
	held, err := models.New(a.DB).ListUserAuthFactors(ctx, models.ListUserAuthFactorsParams{
		UserID: userID, PendingSince: a.Clock.Now().Add(-a.Config.PendingTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("list factors: %w", err)
	}

	visible := make([]models.UserAuthFactor, 0, len(held))

	for _, f := range held {
		if d, ok := a.Factors.Known(f.Type); ok && d.Capabilities().UserVisible {
			visible = append(visible, f)
		}
	}

	return visible, nil
}
