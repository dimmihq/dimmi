/*
|--------------------------------------------------------------------------
| Tag Event Header
|--------------------------------------------------------------------------
|
| Every tag event names the workspace, the time and the member who made
| the change, so open screens and the audit trail agree on who did what.
|
*/

package tags

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
base returns the header of a tag event made by the member.
*/
func base(workspaceID ids.UUID, m authz.Membership, now time.Time) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: workspaceID, OccurredAt: now,
		Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
	}
}
