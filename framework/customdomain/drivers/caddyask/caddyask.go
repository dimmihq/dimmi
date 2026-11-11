/*
|--------------------------------------------------------------------------
| Caddy On-Demand TLS
|--------------------------------------------------------------------------
|
| Caddy can terminate TLS in front of the application and obtain a
| certificate the first time a hostname is visited. Left alone it would
| do so for any hostname pointed at it, so before ordering it asks the
| application over HTTP whether the hostname is allowed:
|
|   on_demand_tls {
|       ask http://app:9090/internal/tls/ask
|   }
|
| The ask endpoint answers from the same authorizer every driver uses:
| 200 to allow, 404 to refuse. Caddy keeps its own certificates, so the
| application has nothing to store or delete.
|
*/

package caddyask

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
AskPath is where the admin listener answers Caddy's questions.
*/
const AskPath = "/internal/tls/ask"

/*
Driver leaves TLS to Caddy and answers its questions.
*/
type Driver struct {
	authorize customdomain.Authorizer
}

var _ customdomain.TLS = (*Driver)(nil)

/*
New returns the driver, allowing Caddy only the hostnames the
authorizer allows.
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
Ensure checks that the hostname is authorized; Caddy obtains the
certificate on the hostname's first visit.
*/
func (d *Driver) Ensure(ctx context.Context, host string) error {
	return customdomain.Authorize(ctx, d.authorize, host)
}

/*
Remove has nothing to delete: once the authorizer refuses the hostname,
Caddy stops renewing its certificate.
*/
func (d *Driver) Remove(context.Context, string) error {
	return nil
}

/*
TLSConfig returns nil: Caddy terminates TLS.
*/
func (d *Driver) TLSConfig() *tls.Config {
	return nil
}

/*
HTTPHandler returns the handler unchanged: Caddy answers challenges.
*/
func (d *Driver) HTTPHandler(next http.Handler) http.Handler {
	return next
}

/*
AskHandler answers Caddy's question about the hostname in the "domain"
query parameter: 200 when the authorizer allows it, 404 when it refuses
or the hostname is invalid. It belongs on an internal port only.
*/
func (d *Driver) AskHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, err := customdomain.Normalize(r.URL.Query().Get("domain"))
		if err == nil {
			err = customdomain.Authorize(r.Context(), d.authorize, host)
		}

		if err != nil {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
