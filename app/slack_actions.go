/*
|--------------------------------------------------------------------------
| Slack Actions
|--------------------------------------------------------------------------
|
| The Slack channel's actions, built once at boot: the channel itself,
| which keeps and processes Slack's deliveries and connects workspaces'
| own apps, the router other features claim Slack's deliveries on, and
| the edits and deletions Slack's users make to their messages.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
SlackActions holds the Slack channel's actions.
*/
type SlackActions struct {
	Channel *channels.Slack
	Router  *channels.Router
	Edits   *conversations.InboundEdits
	Sockets *channels.SocketSupervisor
}
