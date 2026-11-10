/*
|--------------------------------------------------------------------------
| Integration Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a workspace connects a tool, a teammate
| connects their own account on it, or a feature asks for its tokens.
| Each failure the browser lands on after a provider sends it back has a
| code the settings page explains.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Installation failures.

ErrIntegrationUnavailable is a provider this install does not offer or
has no app for. ErrIntegrationFeatureUnknown is a feature the provider
has no scopes for. ErrIntegrationReturnTo is a page to come back to
that is not on this site. ErrIntegrationState is a callback that is
unknown, expired, used, or opened in another browser.
ErrIntegrationDenied is a person who declined at the provider.
ErrIntegrationProvider is a provider that failed or refused the code.
ErrIntegrationAccountTaken is a provider account another workspace
already connected. ErrEnterpriseInstallUnsupported is an Enterprise Grid
organization install, which only works per Slack workspace.
ErrIntegrationAccountMismatch is a token or a teammate's grant for
another account than the installation's. ErrIntegrationNotActive is an
installation that must be reconnected first. ErrIntegrationNoUserScopes
is a provider with nothing a teammate could connect.
ErrIntegrationAuthFailed is pasted credentials the provider refused.
*/
var (
	ErrIntegrationUnavailable = fault.New(fault.NotFound, "integration_unavailable",
		"This integration is not available on this server.")
	ErrIntegrationFeatureUnknown = fault.New(fault.Invalid, "integration_feature_unknown",
		"This integration has no such feature.").OnField("features")
	ErrIntegrationReturnTo = fault.New(fault.Invalid, "integration_return_to_invalid",
		"Return to must be a path on this site.").OnField("return_to")
	ErrIntegrationState = fault.New(fault.Invalid, "integration_state",
		"This connection link expired or was opened elsewhere. Start again.")
	ErrIntegrationDenied   = fault.New(fault.Forbidden, "denied", "The connection was declined.")
	ErrIntegrationProvider = fault.New(fault.FailedDependency, "integration_provider_error",
		"The provider could not complete the connection.")
	ErrIntegrationAccountTaken = fault.New(fault.Conflict, "integration_account_taken",
		"Another dimmi workspace already connected this account.")
	ErrEnterpriseInstallUnsupported = fault.New(fault.Invalid, "enterprise_install_unsupported",
		"Install the app on one Slack workspace, not on the whole organization.")
	ErrIntegrationAccountMismatch = fault.New(fault.Invalid, "integration_account_mismatch",
		"These credentials belong to another account.")
	ErrIntegrationNotActive = fault.New(fault.Conflict, "integration_not_active",
		"Reconnect this integration first.")
	ErrIntegrationNoUserScopes = fault.New(fault.Invalid, "integration_no_user_scopes",
		"Teammates cannot connect their own account on this integration.")
	ErrIntegrationAuthFailed = fault.New(fault.Invalid, "integration_auth_failed",
		"The provider refused these credentials.")
)
