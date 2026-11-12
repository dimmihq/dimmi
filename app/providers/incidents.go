/*
|--------------------------------------------------------------------------
| Incident Wiring
|--------------------------------------------------------------------------
|
| Incidents watch new conversations: every minute the spike detector
| groups the last fifteen minutes of them, and each conversation a
| customer starts is matched to the incidents active now and to open
| trackers once it is saved.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootIncidents registers the spike detector and the matcher of new
conversations.
*/
func bootIncidents(a *app.App) error {
	t := a.Actions.Tickets
	if t == nil {
		return nil
	}

	t.ListenForMatches(a.Events)

	return errors.Join(
		queue.Register(a.Queue, &jobs.DetectIncidentSpikes{Spikes: &tickets.Spikes{Tickets: t}}),
		queue.Register(a.Queue, &jobs.MatchTicketConversation{Work: t}),
	)
}
