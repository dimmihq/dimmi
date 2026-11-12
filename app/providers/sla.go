/*
|--------------------------------------------------------------------------
| Service Level Wiring
|--------------------------------------------------------------------------
|
| Service level clocks follow tickets and conversations, so the action
| hears their events after the tickets action does; a sweep warns about
| and breaches due clocks. The workflow engine applies policies and
| works tickets through these actions.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/sla"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootSLA builds the service level action, hears the work it times and
registers its sweep.
*/
func bootSLA(a *app.App) error {
	s := &sla.SLA{DB: a.DB, Clock: a.Clock, Events: a.Events, Tickets: a.Actions.Tickets}
	s.Listen(a.Events)
	a.Actions.SLA = s

	if engine := a.Actions.WorkflowEngine; engine != nil {
		engine.Tickets, engine.SLA = a.Actions.Tickets, s
	}

	return queue.Register(a.Queue, &jobs.SweepSLAClocks{Clocks: s})
}
