/*
|--------------------------------------------------------------------------
| Chat Channel Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a workspace connects a chat app of its own,
| such as a Slack app created from the manifest. Each says what to fix:
| a token the provider refused, scopes the app was created without, a
| socket the app never enabled, or an app another workspace connected.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Connection failures.

ErrChannelAlreadyConnected is an app and team this workspace connected
already. ErrChannelConnectedElsewhere is one another workspace holds,
never named. ErrSlackTokenRejected is a bot token Slack refused, or one
of another kind, ErrSlackSigningSecretRequired a bot token without its
signing secret, and
ErrSlackScopesMissing an app created without every scope the channel
needs. ErrSlackSocketModeDisabled is an app token that cannot open a
socket. ErrChannelNotSlack is a connection of another type.
ErrChannelUnavailable is a provider Slack could not be reached at.
*/
var (
	ErrChannelAlreadyConnected = fault.New(fault.Conflict, "channel_already_connected",
		"This workspace already connected this app.")
	ErrChannelConnectedElsewhere = fault.New(fault.Conflict, "channel_connected_elsewhere",
		"Another dimmi workspace already connected this app.")
	ErrSlackTokenRejected = fault.New(fault.Invalid, "slack_token_rejected",
		"Slack refused this bot token.").OnField("bot_token")
	ErrSlackSigningSecretRequired = fault.New(fault.Invalid, "slack_signing_secret_required",
		"Paste the app's signing secret with its bot token.").OnField("signing_secret")
	ErrSlackScopesMissing = fault.New(fault.Invalid, "slack_scopes_missing",
		"The Slack app lacks scopes the channel needs. Update it from the manifest and install it again.").
		OnField("bot_token")
	ErrSlackSocketModeDisabled = fault.New(fault.Invalid, "slack_socket_mode_disabled",
		"This app token cannot open a socket. Turn on Socket Mode for the app.").OnField("app_token")
	ErrChannelNotSlack    = fault.New(fault.NotFound, "not_found", "Not Found.")
	ErrChannelUnavailable = fault.New(fault.FailedDependency, "provider_unavailable",
		"Slack could not be reached. Try again shortly.")
)
