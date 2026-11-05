/*
|--------------------------------------------------------------------------
| Look Up Single Sign-On
|--------------------------------------------------------------------------
|
| The sign-in page asks whether an address's company signs in through
| its identity provider. The answer depends on the domain alone, never
| on whether the address has an account, so it tells a stranger nothing
| about who uses dimmi. It is limited per address of the asker.
|
*/

package ssologin

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
LookupInput is the address typed and the client asking.
*/
type LookupInput struct {
	Email  string
	Client appauth.Client
}

/*
Lookup tells where an address signs in.

Errors: [exceptions.ErrTooManyAttempts] past thirty a minute from one
address, [exceptions.ErrAuthBusy] when the answer cannot be read.
*/
type Lookup struct {
	Policy  appauth.OrganizationLoginPolicy
	Limiter *ratelimit.Limiter
}

/*
Handle returns the connection the address's domain routes to, and
whether one does.
*/
func (a *Lookup) Handle(ctx context.Context, in LookupInput) (appauth.SSORoute, bool, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "sso-lookup:ip:" + in.Client.IP, Limit: 30, Window: time.Minute,
	})
	if err != nil {
		return appauth.SSORoute{}, false, err
	}

	return a.Policy.Route(ctx, in.Email)
}
