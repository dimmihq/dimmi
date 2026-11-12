/*
|--------------------------------------------------------------------------
| Certificate Hosts
|--------------------------------------------------------------------------
|
| The framework allows a certificate for the app's own host and asks the
| product about every other one. The product allows its help centers'
| hosts on the help domain.
|
*/

package app

import (
	"context"

	"github.com/dimmihq/dimmi/framework/customdomain"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
AuthorizeDomain reports whether the host may have a TLS certificate.
*/
func (a *App) AuthorizeDomain(ctx context.Context, host string) error {
	if a.Application == nil || a.DB == nil {
		return customdomain.ErrNotAuthorized
	}

	hosts := helpcenters.HelpHosts{
		DB: a.DB, Addresses: helpcenters.Addresses{BaseDomain: a.Config.HelpCenters.BaseDomain},
	}

	return hosts.Authorize(ctx, host)
}
