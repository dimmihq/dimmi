/*
|--------------------------------------------------------------------------
| Help Center Hosts
|--------------------------------------------------------------------------
|
| A help center on its own subdomain of the help domain needs a TLS
| certificate, which the edge asks for the first time someone opens it.
| The edge asks the app first, and only an active help center's host is
| allowed, so nobody can make the edge request certificates for names
| that serve nothing.
|
*/

package helpcenters

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpHosts decides which hosts of the help domain may have a certificate.
*/
type HelpHosts struct {
	DB        *database.DB
	Addresses Addresses
}

/*
Authorize allows the host of an active help center on the help domain
and refuses every other host with customdomain.ErrNotAuthorized.
*/
func (h HelpHosts) Authorize(ctx context.Context, host string) error {
	sub := h.Addresses.Subdomain(host)
	if sub == "" {
		return customdomain.ErrNotAuthorized
	}

	_, err := models.New(h.DB).ResolveHelpCenter(ctx, sub)
	if errors.Is(err, database.ErrNotFound) {
		return customdomain.ErrNotAuthorized
	}

	return err
}
