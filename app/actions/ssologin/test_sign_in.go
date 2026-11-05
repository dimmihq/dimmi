/*
|--------------------------------------------------------------------------
| Start Connection Test
|--------------------------------------------------------------------------
|
| Before a connection signs anyone in, an owner tries it: they sign in at
| the identity provider in a new tab, and a valid answer marks the
| connection tested without starting a session. Only the company's own
| connections can be tried, by someone who manages single sign-on.
|
*/

package ssologin

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StartTestInput is who tries the connection, through their membership,
and which.
*/
type StartTestInput struct {
	Membership   authz.Membership
	UserID       ids.UUID
	ConnectionID ids.UUID
}

/*
StartTest starts an owner's test sign-in through a connection.

Errors: [exceptions.ErrForbidden] without sso:manage,
[exceptions.ErrNotFound] for another company's connection or a disabled
one, or what [StartSignIn] refuses.
*/
type StartTest struct {
	DB    *database.DB
	Start *StartSignIn
}

/*
Handle returns where the owner signs in, and the attempt cookie value.
*/
func (a *StartTest) Handle(ctx context.Context, in StartTestInput) (Started, error) {
	if !in.Membership.Can("sso:manage") {
		return Started{}, exceptions.ErrForbidden
	}

	_, err := models.New(a.DB).FindSSOConnection(ctx, models.FindSSOConnectionParams{
		OrganizationID: in.Membership.Workspace.OrganizationID, ID: in.ConnectionID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return Started{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Started{}, fmt.Errorf("find connection to test: %w", err)
	}

	return a.Start.Handle(ctx, StartSignInInput{
		ConnectionID: in.ConnectionID, Intent: appauth.SSOIntentTest, UserID: in.UserID,
	})
}
