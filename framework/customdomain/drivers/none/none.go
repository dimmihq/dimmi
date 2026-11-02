/*
|--------------------------------------------------------------------------
| TLS Handled Elsewhere
|--------------------------------------------------------------------------
|
| Some operators run their own proxy in front of the application and
| manage certificates there. The application then serves custom domains
| over plain HTTP and has no certificate to obtain or delete; it still
| refuses hostnames the authorizer does not allow, so every driver
| answers Ensure the same way.
|
*/

package none

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Driver serves custom domains without terminating TLS.
*/
type Driver struct {
	authorize customdomain.Authorizer
}

var _ customdomain.TLS = (*Driver)(nil)

/*
New returns the driver, refusing hostnames the authorizer refuses.
*/
func New(authorize customdomain.Authorizer) *Driver {
	return &Driver{authorize: authorize}
}

/*
Factory builds the driver for the TLS manager. It has no options.
*/
func Factory(authorize customdomain.Authorizer) manager.Factory[customdomain.TLS] {
	return func(string, config.Section) (customdomain.TLS, error) {
		return New(authorize), nil
	}
}

/*
Ensure only checks that the hostname is authorized.
*/
func (d *Driver) Ensure(ctx context.Context, host string) error {
	return customdomain.Authorize(ctx, d.authorize, host)
}

/*
Remove has nothing to delete.
*/
func (d *Driver) Remove(context.Context, string) error {
	return nil
}

/*
TLSConfig returns nil: TLS is terminated outside the application.
*/
func (d *Driver) TLSConfig() *tls.Config {
	return nil
}

/*
HTTPHandler returns the handler unchanged.
*/
func (d *Driver) HTTPHandler(next http.Handler) http.Handler {
	return next
}
