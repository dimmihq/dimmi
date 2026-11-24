/*
|--------------------------------------------------------------------------
| A/B Splits
|--------------------------------------------------------------------------
|
| A split sends the conversation down one variant: the winner once the
| team picked one, else the variant a hash of the conversation and the
| step gives it, so it takes the same one every time it meets the step.
| The trace keeps the variant, which is how analytics compare them.
|
*/

package workflows

import (
	"context"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The outcomes of a split.
*/
const (
	outcomeVariant = "variant"
	outcomeWinner  = "winner"
)

/*
runSplit takes the conversation's variant.
*/
func runSplit(_ context.Context, st *state, s workflowspec.Step) (outcome, error) {
	key := st.run.ID.String()
	if st.run.ConversationID != nil {
		key = st.run.ConversationID.String()
	}

	return splitOutcome(s, key), nil
}

/*
splitOutcome is the path a conversation key takes at a split, the same
for a run and a dry run.
*/
func splitOutcome(s workflowspec.Step, key string) outcome {
	v, winner := settings[workflowspec.SplitSettings](s).Pick(key, s.ID)
	out := outcome{path: v.ID, outcome: outcomeVariant, detail: map[string]any{"variant": v.ID, "label": v.Label}}
	if winner {
		out.outcome = outcomeWinner
	}

	return out
}
