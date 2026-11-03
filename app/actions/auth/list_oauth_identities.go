/*
|--------------------------------------------------------------------------
| List OAuth Identities
|--------------------------------------------------------------------------
|
| The provider accounts linked to a user, so they can see every way into
| their account and remove one they do not recognize.
|
*/

package auth

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ListOAuthIdentities returns a user's linked provider accounts, oldest
first.

Errors: a wrapped database error.
*/
type ListOAuthIdentities struct {
	DB *database.DB
}

/*
Handle lists the identities.
*/
func (a *ListOAuthIdentities) Handle(ctx context.Context, userID ids.UUID) ([]models.OauthIdentity, error) {
	identities, err := models.New(a.DB).ListUserOAuthIdentities(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list identities: %w", err)
	}

	return identities, nil
}
