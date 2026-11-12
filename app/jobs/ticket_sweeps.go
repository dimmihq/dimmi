/*
|--------------------------------------------------------------------------
| Ticket Sweeps
|--------------------------------------------------------------------------
|
| The periodic ticket sweeps: tickets that waited on their customer too
| long resolve, and service level clocks that are due warn or breach.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
SweepTicketsArgs resolves tickets that waited on the customer too long.
*/
type SweepTicketsArgs struct{}

/*
SweepSLAClocksArgs warns about and breaches due service level clocks.
*/
type SweepSLAClocksArgs struct{}

/*
SweepTickets resolves tickets that waited too long.
*/
type SweepTickets struct {
	Work TicketWork
}

/*
Handle runs the sweep.
*/
func (w *SweepTickets) Handle(ctx context.Context, _ *queue.Job[SweepTicketsArgs]) error {
	return w.Work.Sweep(ctx)
}

/*
Sweeper runs one periodic sweep.
*/
type Sweeper interface {
	/*
		Sweep does one round of the periodic work.
	*/
	Sweep(ctx context.Context) error
}

/*
SweepSLAClocks warns about and breaches due clocks.
*/
type SweepSLAClocks struct {
	Clocks Sweeper
}

/*
Handle runs the sweep.
*/
func (w *SweepSLAClocks) Handle(ctx context.Context, _ *queue.Job[SweepSLAClocksArgs]) error {
	return w.Clocks.Sweep(ctx)
}
