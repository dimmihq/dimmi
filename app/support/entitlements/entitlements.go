/*
|--------------------------------------------------------------------------
| Entitlements Catalog
|--------------------------------------------------------------------------
|
| Every cap in the product is a key here, with the value it has on a
| self-hosted install and on the cloud. A key nobody listed is a mistake in
| the code that asks for it, and fails loudly instead of quietly allowing
| everything. Limits can be raised for one workspace by a driver that
| knows about overrides; existing rows are never touched when a limit drops.
|
*/

package entitlements

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The keys of the catalog.
*/
const (
	KeyWidgetMaxPerWorkspace = "widget.max_per_workspace"
	KeyBillingTestModeCap    = "billing.test_mode_cap"
)

/*
ErrUnknownKey means the code asked about a key the catalog does not list.
*/
var ErrUnknownKey = errors.New("entitlements: unknown key")

/*
Unlimited is the limit of something with no cap.
*/
const Unlimited = -1

/*
Catalog is the value of each key on an install.
*/
type Catalog map[string]int64

/*
SelfHost returns the catalog of a self-hosted install, where the operator
sets the number of widgets and zero means no limit.
*/
func SelfHost(maxWidgets int64) Catalog {
	if maxWidgets == 0 {
		maxWidgets = Unlimited
	}

	return Catalog{KeyWidgetMaxPerWorkspace: maxWidgets, KeyBillingTestModeCap: Unlimited}
}

/*
Cloud returns the catalog of the cloud.
*/
func Cloud() Catalog {
	return Catalog{KeyWidgetMaxPerWorkspace: 10, KeyBillingTestModeCap: 1000}
}

/*
Driver answers entitlements from a catalog, the same for every workspace.
*/
type Driver struct {
	Catalog Catalog
}

/*
Has answers yes: nothing in the product is behind a paywall.
*/
func (d Driver) Has(context.Context, ids.UUID, string) (bool, error) {
	return true, nil
}

/*
Limit returns the catalog's value of the key.
*/
func (d Driver) Limit(_ context.Context, _ ids.UUID, key string) (int64, error) {
	limit, ok := d.Catalog[key]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}

	return limit, nil
}
