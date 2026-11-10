/*
|--------------------------------------------------------------------------
| Token Errors
|--------------------------------------------------------------------------
|
| A feature that asks for a token learns why it cannot have one, so it
| fails the right way: a disconnected tool stops for good, with no retry
| storm, a broken one waits for a teammate to reconnect it, and a
| teammate without their own grant falls back to acting as the app.
|
*/

package integrations

import "errors"

/*
Errors of the token service.

ErrIntegrationDisconnected is an installation that is gone, removed or
uninstalled at the provider. ErrIntegrationBroken is an installation
whose tokens the provider refused; a teammate must reconnect it.
ErrNoMemberGrant is a member who has no working grant of their own on
the installation.
*/
var (
	ErrIntegrationDisconnected = errors.New("integrations: installation disconnected")
	ErrIntegrationBroken       = errors.New("integrations: installation broken")
	ErrNoMemberGrant           = errors.New("integrations: member has no grant")
)
