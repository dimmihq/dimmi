/*
|--------------------------------------------------------------------------
| Slack Ticket Intake Wiring
|--------------------------------------------------------------------------
|
| A teammate turns a customer's Slack thread into a ticket with the
| ticket emoji or a message shortcut. The intake claims both on the
| Slack channel's router once the channel and tickets are built.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/slacktickets"
)

/*
bootSlackTickets claims the ticket emoji and shortcut on Slack, when
the install runs the Slack channel.
*/
func bootSlackTickets(a *app.App) {
	if a.Actions.Slack == nil || a.Actions.Tickets == nil {
		return
	}

	intake := &slacktickets.Intake{
		DB: a.DB, Clock: a.Clock, Events: a.Events, IDs: a.IDs, Slack: a.Actions.Slack.Channel,
		Tickets: a.Actions.Tickets, AppURL: a.Application.Config.App.URL,
	}
	intake.Claim(a.Actions.Slack.Router)
}
