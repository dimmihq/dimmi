/*
|--------------------------------------------------------------------------
| Help Center Origins
|--------------------------------------------------------------------------
|
| A help center on its own host of the help domain loads the workspace's
| messenger from the app. A messenger kept to a list of sites still
| answers its own workspace's help center, so the team never has to list
| an address it did not choose; any other help center stays refused.
|
*/

package helpcenters

import (
	"context"
	"net/url"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpOrigins recognizes the sites of a workspace's own help centers.
*/
type HelpOrigins struct {
	DB        *database.DB
	Addresses Addresses
}

/*
Owns reports whether the origin is the host of one of the workspace's
help centers on the help domain.
*/
func (o HelpOrigins) Owns(ctx context.Context, workspaceID ids.UUID, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	sub := o.Addresses.Subdomain(parsed.Host)
	if sub == "" {
		return false
	}
	row, err := models.New(o.DB).ResolveHelpCenter(ctx, sub)
	return err == nil && row.WorkspaceID == workspaceID
}
