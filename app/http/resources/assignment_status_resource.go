/*
|--------------------------------------------------------------------------
| Assignment Status Resource
|--------------------------------------------------------------------------
|
| Why an inbox assigns as it does right now: each member of its pool with
| what they hold against their limits and why they may not take the next
| conversation, and the conversations waiting, summed up by reason.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AssignmentStatus is an inbox's automatic assignment as it stands.
*/
type AssignmentStatus struct {
	Object  string                 `json:"object"`
	Mode    string                 `json:"mode"`
	Channel string                 `json:"channel_type"`
	Members []AssignmentPoolMember `json:"members"`
	Waiting AssignmentWaiting      `json:"waiting"`
}

/*
AssignmentPoolMember is one member of the pool: open conversations held,
the limits that apply, whether they may take the next one, and why not:
away, at_capacity, at_channel_capacity or null.
*/
type AssignmentPoolMember struct {
	Member          Named   `json:"member"`
	Availability    string  `json:"availability"`
	OpenCount       int32   `json:"open_count"`
	Capacity        *int    `json:"capacity"`
	ChannelCapacity *int    `json:"channel_capacity"`
	Eligible        bool    `json:"eligible"`
	WhyNot          *string `json:"why_not"`
}

/*
AssignmentWaiting sums up the waiting conversations.
*/
type AssignmentWaiting struct {
	Count    int            `json:"count"`
	OldestAt *time.Time     `json:"oldest_at"`
	Reasons  map[string]int `json:"reasons"`
}

/*
NewAssignmentStatus shapes the status.
*/
func NewAssignmentStatus(s assignment.Status) AssignmentStatus {
	members := make([]AssignmentPoolMember, len(s.Members))

	for i, c := range s.Members {
		members[i] = AssignmentPoolMember{
			Member:       Named{ID: models.PublicID(models.PrefixMember, c.ID), Object: "member", Name: s.Names[c.ID]},
			Availability: c.Availability, OpenCount: c.OpenCount, Capacity: c.Capacity,
			ChannelCapacity: c.ChannelCapacity, Eligible: c.WhyNot == "",
		}

		if c.WhyNot != "" {
			members[i].WhyNot = &c.WhyNot
		}
	}

	return AssignmentStatus{
		Object: "assignment_status", Mode: s.Mode, Channel: string(s.ChannelType), Members: members,
		Waiting: AssignmentWaiting{Count: s.Waiting.Count, OldestAt: s.Waiting.OldestAt, Reasons: s.Waiting.Reasons},
	}
}
