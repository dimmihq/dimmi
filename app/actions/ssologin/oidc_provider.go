/*
|--------------------------------------------------------------------------
| OpenID Connect Provider Of A Connection
|--------------------------------------------------------------------------
|
| An OpenID Connect connection signs people in through its issuer with
| the client it registered there.
|
*/

package ssologin

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
OIDCProviders builds the OpenID Connect side of a connection.
*/
type OIDCProviders struct{}

/*
AuthURL returns the issuer's sign-in address for the attempt.
*/
func (p OIDCProviders) AuthURL(_ context.Context, _ sso.Connection, _ string, _ *appauth.SSOAttempt) (string, error) {
	return "", exceptions.ErrNotFound
}
