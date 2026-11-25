/*
|--------------------------------------------------------------------------
| Confirmed Resolution
|--------------------------------------------------------------------------
|
| Inside a workflow's agent step that asks the customer to confirm, the
| agent ends its answer by asking whether it solved the problem. When the
| customer writes back right after, the decision model judges whether
| the reply confirms it, in any language or wording. A confirmation ends
| the agent's turn without a reply and tells the workflow, which takes
| its resolved path. Anything else is a new turn the agent answers. The
| bar is high: calling an unsolved problem solved costs the customer.
|
*/

package pipeline

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
skipResolutionConfirmed is the skip reason of a turn the customer ended
by confirming the agent's answer solved the problem.
*/
const skipResolutionConfirmed = "resolution_confirmed"

/*
confirmCut bounds how much of the answer and the reply the judge reads.
*/
const confirmCut = 1500

/*
confirmsResolution reports whether the customer's pending reply confirms
the agent's answer right before it solved the problem, inside an agent
step that asks for confirmation.
*/
func (r *Runner) confirmsResolution(ctx context.Context, st *run) bool {
	e := st.env
	if !st.settings.Workflow.StepActive || !st.settings.ConfirmResolution || len(e.History) == 0 {
		return false
	}

	last := e.History[len(e.History)-1]
	if last.AuthorType != string(enums.AuthorTypeAgent) || last.Body == nil {
		return false
	}

	reply := strings.TrimSpace(e.pendingText())
	if reply == "" {
		return false
	}

	vars := prompts.Vars{"answer": truncate(*last.Body, confirmCut), "reply": truncate(reply, confirmCut)}
	answers := r.judge(ctx, st, contracts.DecisionRequest{
		State:     prompts.Judgment("confirms_resolution", vars).State,
		Questions: map[string]contracts.DecisionQuestion{"confirms_resolution": question("confirms_resolution", vars)},
	})

	return answers["confirms_resolution"] >= sure
}

/*
writeResolved tells the workflow holding the conversation that the
customer confirmed the agent's answer solved the problem.
*/
func (r *Runner) writeResolved(ctx context.Context, tx *database.Tx, st *run) error {
	if r.Events == nil {
		return nil
	}

	e := st.env

	return r.Events.Dispatch(ctx, tx, appevents.AiResolutionConfirmed{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: e.Workspace.ID, OccurredAt: r.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeContact), ID: e.Conversation.ContactID},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: e.Conversation.ID},
	})
}
