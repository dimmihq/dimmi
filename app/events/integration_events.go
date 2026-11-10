/*
|--------------------------------------------------------------------------
| Integration Events
|--------------------------------------------------------------------------
|
| One installation of a tool feeds every feature that uses it, so each
| feature reacts to the installation's life through these events rather
| than polling: a channel is created when Slack connects for channels,
| destinations stop when Slack is uninstalled. No event ever carries a
| token; a listener that needs one asks the token service.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
IntegrationConnected is dispatched in the transaction that connected an
installation, reconnected it, or added features to one that already had
their scopes. Features are the features the install was for, Scopes
what the installation holds now, and Source oauth or manual.
Reconnected says an earlier installation of the same account came back.
*/
type IntegrationConnected struct {
	Base
	InstallationID    ids.UUID
	Provider          string
	ExternalAccountID string
	ExternalAppID     string
	Source            string
	Features          []string
	Scopes            []string
	Reconnected       bool
}

/*
EventName names the event.
*/
func (IntegrationConnected) EventName() string {
	return "integration.connected"
}

/*
IntegrationErrored is dispatched in the transaction that found an
installation broken. Code says why, such as refresh_invalid_grant or
slack_auth_failed.
*/
type IntegrationErrored struct {
	Base
	InstallationID ids.UUID
	Provider       string
	Code           string
}

/*
EventName names the event.
*/
func (IntegrationErrored) EventName() string {
	return "integration.errored"
}

/*
IntegrationDisconnected is dispatched in the transaction that
disconnected an installation. Reason is uninstalled or revoked when the
provider reported it, disconnected when a teammate did.
*/
type IntegrationDisconnected struct {
	Base
	InstallationID ids.UUID
	Provider       string
	Reason         string
}

/*
EventName names the event.
*/
func (IntegrationDisconnected) EventName() string {
	return "integration.disconnected"
}

/*
IntegrationScopesChanged is dispatched in the transaction that stored a
new grant whose scopes differ from the installation's earlier ones.
*/
type IntegrationScopesChanged struct {
	Base
	InstallationID ids.UUID
	Provider       string
	Added          []string
	Removed        []string
}

/*
EventName names the event.
*/
func (IntegrationScopesChanged) EventName() string {
	return "integration.scopes_changed"
}
