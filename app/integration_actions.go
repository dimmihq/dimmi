/*
|--------------------------------------------------------------------------
| Integration Actions
|--------------------------------------------------------------------------
|
| Connecting the tools a team already uses, built once at boot with the
| registry of providers and the scopes every feature registered. Tokens
| is the only way a feature gets a token for an installation.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
)

/*
IntegrationActions holds the integrations' actions and services.
*/
type IntegrationActions struct {
	Registry          *oauth.Registry
	Tokens            *integrations.Tokens
	List              *integrations.ListInstallations
	StartInstallation *integrations.StartInstallation
	CompleteCallback  *integrations.CompleteCallback
	CreateManual      *integrations.CreateManualInstallation
	StartMemberGrant  *integrations.StartMemberGrant
	RevokeMemberGrant *integrations.RevokeMemberGrant
	Disconnect        *integrations.DisconnectInstallation
	ApplyRevocation   *integrations.ApplyProviderRevocation
	CheckHealth       *integrations.CheckInstallationHealth
	RevokeDeparted    *integrations.RevokeDepartedMember
}
