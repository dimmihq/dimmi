/*
|--------------------------------------------------------------------------
| Availability Broadcasts
|--------------------------------------------------------------------------
|
| Teammates see at a glance who is away: a change of someone's
| availability reaches everyone looking at the workspace, which then
| reads the member again. The broadcast names the member only, so what
| being away means for their conversations stays with those who may read
| members.
|
*/

package broadcasting

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
listenAvailability announces each change of a member's availability,
once it commits.
*/
func (b *Broadcaster) listenAvailability(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberAvailabilityChanged) error {
		return b.member(ctx, e.WorkspaceID, "member.availability_changed", e.MemberID)
	})
}
