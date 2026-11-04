/*
|--------------------------------------------------------------------------
| Snooze List Events
|--------------------------------------------------------------------------
|
| A change to the snooze choices reaches every open screen of the
| workspace, so the next snooze offers what the admins just set.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
SnoozeListUpdated is dispatched when the workspace's snooze list, or the
team's when TeamID is set, is set or removed.
*/
type SnoozeListUpdated struct {
	Base

	TeamID *ids.UUID
}

/*
EventName names the event.
*/
func (SnoozeListUpdated) EventName() string { return "snooze_list.updated" }
