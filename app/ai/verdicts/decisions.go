/*
|--------------------------------------------------------------------------
| Decision Steps
|--------------------------------------------------------------------------
|
| The AI decision step workflows, series and conversation attributes
| share asks the decision model first; the workspace's chat model is
| what it falls back on, and that is the verdicts' cheap model here. The
| classifier model is asked, and asked again when it fails; then the
| answer model, usually on another provider, is asked in its place.
| Each try is its own run of the classify kind, so its cost shows and
| counts on the month like every other verdict.
|
*/

package verdicts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
)

/*
modelTries is how often the first model is asked before the next one.
*/
const modelTries = 2

/*
decisionTokens bounds a decision step's answer, a few values at most.
*/
const decisionTokens = 600

/*
AskModel asks the workspace's models in turn until one answers.
*/
func (v *Verdicts) AskModel(ctx context.Context, q aidecision.ModelQuestion, out any) error {
	if _, ok := tenancy.From(ctx); !ok {
		ctx = tenancy.With(ctx, q.WorkspaceID.String())
	}

	ask := question{
		workspaceID: q.WorkspaceID, kind: "classify", conversationID: q.ConversationID, system: q.System,
		text: q.Context, schemaName: q.SchemaName, schema: q.Schema, maxTokens: decisionTokens,
	}

	all, err := v.chooseAll(ctx, ask)
	if err != nil {
		return err
	}

	var failed error

	for i, a := range all {
		tries := 1
		if i == 0 {
			tries = modelTries
		}

		for range tries {
			err := v.askWith(ctx, ask, a, out)
			if err == nil {
				return nil
			}

			failed = errors.Join(failed, err)
			if errors.Is(err, ErrNoModel) {
				break
			}
		}
	}

	return failed
}
