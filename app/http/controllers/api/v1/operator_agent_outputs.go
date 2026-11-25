/*
|--------------------------------------------------------------------------
| Operator Agent Outputs
|--------------------------------------------------------------------------
|
| What the routes for agents doing Operator's work answer with about
| suggestions and a routine's state.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
OperatorSuggestionOutcomeOutput is what came of a suggestion.
*/
type OperatorSuggestionOutcomeOutput struct {
	Body fhttp.Resource[resources.OperatorSuggestionOutcome]
}

/*
OperatorRecentSuggestionsOutput is the suggestions made lately.
*/
type OperatorRecentSuggestionsOutput struct {
	Body fhttp.Collection[resources.OperatorRecentSuggestion]
}

/*
OperatorRoutineStateOutput is a routine's saved state.
*/
type OperatorRoutineStateOutput struct {
	Body fhttp.Resource[resources.OperatorRoutineState]
}

/*
OperatorNothingToChangeOutput says a quiet run was recorded.
*/
type OperatorNothingToChangeOutput struct {
	Body fhttp.Resource[resources.OperatorNothingToChange]
}
