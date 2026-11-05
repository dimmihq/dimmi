/*
|--------------------------------------------------------------------------
| Set Member Availability
|--------------------------------------------------------------------------
|
| Anyone may go away and come back on their own; changing someone else's
| status, or how many conversations anyone holds, is for those who manage
| availability, and is written to the audit log. Coming back online ends
| everything being away meant, so a member never returns still handing
| their conversations back. Reassigned conversations stay where they went.
|
| A change is worked out against the member as they are, inside the
| transaction that changes the rest of the member, and announced only
| when something really changed.
|
*/

package availability

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limit is a requested capacity: Max is the most open conversations, nil
for no limit.
*/
type Limit struct {
	Max *int32
}

/*
Change is a requested change of a member's availability: each field is
nil when it stays as it is. CapacityByChannel replaces the whole map.
*/
type Change struct {
	Availability      *enums.Availability
	AwayReassign      *bool
	AssignOnReply     *enums.AssignOnReply
	Capacity          *Limit
	CapacityByChannel *map[string]int
}

/*
Plan checks the change against the caller's power and works out the
availability the member ends with, nil when it changes nothing.

Rules:
  - Members change their own status and reply rule with
    conversations:write; anyone else's needs members:manage_availability.
  - Capacity always needs members:manage_availability.
  - Online clears reassigning; asking to reassign while online is refused.

Errors: [exceptions.ErrForbidden], [exceptions.ErrAwayReassignOnline].
*/
func Plan(m authz.Membership, target models.WorkspaceMember, c *Change) (*appevents.AvailabilityState, error) {
	if c == nil {
		return nil, nil
	}

	if !m.Can(permission(m, target, c)) {
		return nil, exceptions.ErrForbidden
	}

	from := Current(target)
	to := next(from, c)

	if to.Availability == string(enums.AvailabilityOnline) {
		if c.AwayReassign != nil && *c.AwayReassign {
			return nil, exceptions.ErrAwayReassignOnline
		}

		to.AwayReassign = false
	}

	if equal(from, to) {
		return nil, nil
	}

	return &to, nil
}

/*
permission returns what the caller needs for the change: their own
status and reply rule are theirs to set, everything else is managed.
*/
func permission(m authz.Membership, target models.WorkspaceMember, c *Change) string {
	if m.Member.ID == target.ID && c.Capacity == nil && c.CapacityByChannel == nil {
		return "conversations:write"
	}

	return "members:manage_availability"
}

/*
Current returns the member's availability as it is.
*/
func Current(m models.WorkspaceMember) appevents.AvailabilityState {
	byChannel := map[string]int{}
	_ = json.Unmarshal(m.CapacityByChannel, &byChannel)

	return appevents.AvailabilityState{
		Availability: m.Availability, AwayReassign: m.AwayReassign, AssignOnReply: m.AssignOnReply,
		Capacity: m.ConversationCapacity, CapacityByChannel: byChannel,
	}
}

/*
next returns the availability the change leaves the member with.
*/
func next(from appevents.AvailabilityState, c *Change) appevents.AvailabilityState {
	to := from
	to.CapacityByChannel = maps.Clone(from.CapacityByChannel)

	if c.Availability != nil {
		to.Availability = string(*c.Availability)
	}

	if c.AwayReassign != nil {
		to.AwayReassign = *c.AwayReassign
	}

	if c.AssignOnReply != nil {
		to.AssignOnReply = string(*c.AssignOnReply)
	}

	if c.Capacity != nil {
		to.Capacity = c.Capacity.Max
	}

	if c.CapacityByChannel != nil {
		to.CapacityByChannel = maps.Clone(*c.CapacityByChannel)
	}

	return to
}

/*
equal reports whether two availabilities are the same.
*/
func equal(a, b appevents.AvailabilityState) bool {
	sameCapacity := (a.Capacity == nil) == (b.Capacity == nil) && (a.Capacity == nil || *a.Capacity == *b.Capacity)

	return a.Availability == b.Availability && a.AwayReassign == b.AwayReassign &&
		a.AssignOnReply == b.AssignOnReply && sameCapacity && maps.Equal(a.CapacityByChannel, b.CapacityByChannel)
}

/*
Writer stores availability changes inside a transaction, on behalf of
the member making them, at one moment.
*/
type Writer struct {
	Tx     *database.Tx
	Events *events.Dispatcher
	By     ids.UUID
	Now    time.Time
}

/*
Write stores the member's new availability and announces it. Going away
starts the away clock; coming back stops it.
*/
func (w Writer) Write(
	ctx context.Context, m models.WorkspaceMember, to appevents.AvailabilityState,
) (models.WorkspaceMember, error) {
	byChannel, err := json.Marshal(to.CapacityByChannel)
	if err != nil {
		return m, fmt.Errorf("encode capacity by channel: %w", err)
	}

	member, err := models.New(w.Tx).SetMemberAvailability(ctx, models.SetMemberAvailabilityParams{
		Availability: to.Availability, AwayReassign: to.AwayReassign, AwaySince: w.awaySince(m, to),
		AvailabilitySource: "manual", AssignOnReply: to.AssignOnReply, ConversationCapacity: to.Capacity,
		CapacityByChannel: byChannel, Now: w.Now, WorkspaceID: m.WorkspaceID, ID: m.ID,
	})
	if err != nil {
		return m, fmt.Errorf("set member availability: %w", err)
	}

	return member, w.Events.Dispatch(ctx, w.Tx, appevents.MemberAvailabilityChanged{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, UserID: m.UserID, From: Current(m), To: to, ChangedBy: w.By,
	})
}

/*
awaySince returns since when the member is away: now when they just
left, unchanged while they stay away, nil once online.
*/
func (w Writer) awaySince(m models.WorkspaceMember, to appevents.AvailabilityState) *time.Time {
	if to.Availability != string(enums.AvailabilityAway) {
		return nil
	}

	if m.Availability == string(enums.AvailabilityAway) && m.AwaySince != nil {
		return m.AwaySince
	}

	now := w.Now

	return &now
}
