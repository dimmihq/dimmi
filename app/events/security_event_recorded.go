/*
|--------------------------------------------------------------------------
| Security Event Recorded
|--------------------------------------------------------------------------
|
| An entry was added to a person's own security log: a sign-in, a failed
| attempt, a change to how they sign in. Their workspaces' admins see the
| ones that matter to them, so the entry travels on with what it holds.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
SecurityEventRecorded is dispatched in the transaction that wrote the
entry: whose log, the entry's type, the address, browser and device it
came from, its details and when it happened.
*/
type SecurityEventRecorded struct {
	UserID      ids.UUID
	Type        string
	IP          string
	UserAgent   string
	DeviceLabel string
	Metadata    map[string]any
	At          time.Time
}
