/*
|--------------------------------------------------------------------------
| Release Held Deliveries
|--------------------------------------------------------------------------
|
| A suspended workspace's chat app deliveries are kept but held. The
| thaw queues this job in its own transaction, so a workspace back in
| business always has its held deliveries processed, oldest first.
|
*/

package workspaces

import (
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
releaseAttempts is how often a release is tried.
*/
const releaseAttempts = 5

/*
ReleaseHeldInboundEventsArgs names the workspace whose held deliveries
are released.
*/
type ReleaseHeldInboundEventsArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
}

/*
Kind names the job.
*/
func (ReleaseHeldInboundEventsArgs) Kind() string { return "release_held_inbound_events" }

/*
Options retry the job a few times, once per workspace at a time.
*/
func (ReleaseHeldInboundEventsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: releaseAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}
