/*
|--------------------------------------------------------------------------
| Show Access Policy
|--------------------------------------------------------------------------
|
| Everything the settings page needs to edit a workspace's rules: the
| rules as they stand, which ways of signing in this install offers at
| all, the address the caller is seen at, and how many live tokens the
| API allowlist would stop, so an admin finds out before their build does.
|
*/

package accesspolicy

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
readSettings is the permission to see a workspace's rules.
*/
const readSettings = "settings:read"

/*
Policy is a workspace's rules with what the settings page shows around
them. Available lists the ways of signing in this install offers, YourIP
is the caller's address and TokensOutside counts live tokens last used
from outside the API allowlist.
*/
type Policy struct {
	Settings      models.SecuritySettings
	Available     []string
	YourIP        netip.Addr
	TokensOutside int
}

/*
ShowAccessPolicy reads a workspace's rules.

Rules:
  - The caller needs settings:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ShowAccessPolicy struct {
	DB        *database.DB
	Clock     clock.Clock
	Providers providers.ProviderResolver
	MagicLink bool
}

/*
Handle returns the rules of the workspace in play for the caller.
*/
func (a *ShowAccessPolicy) Handle(ctx context.Context, p *appauth.Principal, m authz.Membership) (Policy, error) {
	if !m.Can(readSettings) {
		return Policy{}, exceptions.ErrForbidden
	}

	return a.policyOf(ctx, a.DB, p, m.Workspace)
}

/*
policyOf builds the view of the workspace's rules.
*/
func (a *ShowAccessPolicy) policyOf(
	ctx context.Context, db models.DBTX, p *appauth.Principal, w models.Workspace,
) (Policy, error) {
	settings := w.Security()

	outside, err := a.tokensOutside(ctx, db, w.ID, settings.APIAllowlist)
	if err != nil {
		return Policy{}, err
	}

	return Policy{
		Settings: settings, Available: a.available(ctx, settings), YourIP: p.IP, TokensOutside: outside,
	}, nil
}

/*
available lists the ways of signing in this install offers: a password
always, a magic link when mail sign-in is on, each configured provider,
and single sign-on once the organization has an active connection.
*/
func (a *ShowAccessPolicy) available(ctx context.Context, settings models.SecuritySettings) []string {
	methods := []string{appauth.MethodPassword}

	if a.MagicLink {
		methods = append(methods, appauth.MethodMagicLink)
	}

	for _, provider := range a.Providers.Offered(ctx) {
		methods = append(methods, provider.Key())
	}

	if len(settings.SSOConnectionIDs) > 0 {
		methods = append(methods, appauth.MethodSSO)
	}

	return slices.Compact(methods)
}

/*
tokensOutside counts live tokens of the workspace last used from outside
the list. An empty list binds no token.
*/
func (a *ShowAccessPolicy) tokensOutside(
	ctx context.Context, db models.DBTX, workspaceID ids.UUID, list []models.AllowedAddress,
) (int, error) {
	if len(list) == 0 {
		return 0, nil
	}

	addresses, err := models.New(db).ListLiveTokenAddresses(ctx, models.ListLiveTokenAddressesParams{
		WorkspaceID: workspaceID, Now: a.Clock.Now(),
	})
	if err != nil {
		return 0, fmt.Errorf("list token addresses: %w", err)
	}

	outside := 0

	for _, address := range addresses {
		addr, err := netip.ParseAddr(address)
		if err == nil && models.AddressAllowed(list, addr) {
			continue
		}

		outside++
	}

	return outside, nil
}
