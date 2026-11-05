/*
|--------------------------------------------------------------------------
| Member Availability Changed
|--------------------------------------------------------------------------
|
| A member went away, came back, or changed what being away means for
| their conversations. Teammates' screens show the new status, waiting
| conversations get another chance at an assignee when someone comes
| back, and a change made by someone else is written to the audit log.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
AvailabilityState is one state of a member's availability: online or
away, whether customers' answers go back to the pool while away, their
own rule for taking conversations by replying, and how many open
conversations they hold at most, overall and per channel type.
*/
type AvailabilityState struct {
	Availability      string
	AwayReassign      bool
	AssignOnReply     string
	Capacity          *int32
	CapacityByChannel map[string]int
}

/*
MemberAvailabilityChanged is dispatched in the transaction that changed
the member's availability. ChangedBy is the member who changed it,
ids.Nil() when nobody did.
*/
type MemberAvailabilityChanged struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	From        AvailabilityState
	To          AvailabilityState
	ChangedBy   ids.UUID
}

/*
CameBack reports whether the member came back online.
*/
func (e MemberAvailabilityChanged) CameBack() bool {
	return e.From.Availability != e.To.Availability && e.To.Availability == "online"
}
