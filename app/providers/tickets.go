/*
|--------------------------------------------------------------------------
| Ticket Wiring
|--------------------------------------------------------------------------
|
| Tickets follow their conversations, so the action hears conversation
| events; it sends customers notices and tracker messages from jobs,
| resolves tickets that waited too long from a sweep, matches each
| conversation a customer starts to open trackers once it is saved, and
| starts a customer ticket's conversation through the inbox's own
| action.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootTickets builds the tickets action, hears conversations and
registers its jobs.
*/
func bootTickets(a *app.App) error {
	delivery := conversations.Delivery{Channels: a.Channels, Queue: a.Queue}
	t := &tickets.Tickets{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue, Delivery: delivery,
		Starter: &conversations.CreateConversation{DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery},
	}
	t.Listen(a.Events)
	t.ListenForMatches(a.Events)
	a.Actions.Tickets = t

	return errors.Join(
		queue.Register(a.Queue, &jobs.NotifyTicketState{Work: t}),
		queue.Register(a.Queue, &jobs.SendTicketUpdate{Work: t}),
		queue.Register(a.Queue, &jobs.SweepTickets{Work: t}),
		queue.Register(a.Queue, &jobs.MatchTicketConversation{Work: t}),
		bootSLA(a),
	)
}
