/*
|--------------------------------------------------------------------------
| Factor Sealing
|--------------------------------------------------------------------------
|
| A factor's secret is sealed with the install's own data keys, bound to
| the factor's id, and opens only when that id comes back with it.
| Someone able to write the table but not to read the keys cannot move
| one user's authenticator secret onto another user's row: the copy
| fails to open instead of letting them in. The keys belong to the
| install rather than a workspace, since a user's factors follow them
| into every workspace they join.
|
*/

package factors

import (
	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ScopeSystem is the kind of the scope whose data keys belong to the
install itself.
*/
const ScopeSystem = "system"

/*
bind returns a sealer on the install's data keys whose values open only
for the factor id.
*/
func bind(keys encryption.KeyManager, id ids.UUID) twofactor.KeySealer {
	return twofactor.KeySealer{Keys: keys, Scope: encryption.Scope{Kind: ScopeSystem}, AAD: []byte(id.String())}
}
