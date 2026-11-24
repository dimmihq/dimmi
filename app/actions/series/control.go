/*
|--------------------------------------------------------------------------
| Control Group
|--------------------------------------------------------------------------
|
| A series with a goal can hold a share of its entrants out. Each live
| entrant is put in the control group or not once, before the first
| step, by the random part of its enrollment id, so a retried job
| always decides the same. The control group receives nothing: the
| journey ends at once with the reason "control", and the sweep still
| marks the goal when they reach it inside the window. Comparing the
| two groups' goal rates shows what the series added.
|
*/

package series

import (
	"context"
	"encoding/binary"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
MaxControlPercent is the largest share of entrants a control group may
hold.
*/
const MaxControlPercent = 50

/*
ControlKey is the key of the group in an enrollment's variants; step ids
cannot start with an underscore, so it never meets a split's.
*/
const ControlKey = "_control"

/*
The groups an entrant is put in.
*/
const (
	GroupControl = "control"
	GroupTreated = "treated"
)

/*
holdOut puts a live entrant in a group when the series has a control
group, and ends the journey of one held out; held is true when it did.
*/
func (r *run) holdOut(ctx context.Context) (bool, error) {
	goal := r.g.spec.Goal
	if goal == nil || goal.ControlPercent <= 0 || r.e.Test || r.e.NodeID != KindEntry || r.e.StepsExecuted > 0 ||
		r.e.Status != "active" {
		return false, nil
	}

	if _, ok := r.variants[ControlKey]; ok {
		return false, nil
	}

	if !InControl(r.e.ID, goal.ControlPercent) {
		r.variants[ControlKey] = GroupTreated

		return false, nil
	}

	r.variants[ControlKey] = GroupControl

	return true, r.end(ctx, GroupControl)
}

/*
InControl reports whether an enrollment falls in a control group of
the percent, by the random part of its id.
*/
func InControl(id ids.UUID, percent int) bool {
	if percent <= 0 {
		return false
	}

	return int(binary.BigEndian.Uint64(id[8:])%100) < min(percent, MaxControlPercent)
}
