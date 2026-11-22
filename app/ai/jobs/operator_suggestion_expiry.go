/*
|--------------------------------------------------------------------------
| Stale Operator Suggestions
|--------------------------------------------------------------------------
|
| Each night the routine suggestions nobody decided within four weeks
| are declined, so review queues show only what still matters.
|
*/

package aijobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/ai/operator"
)

/*
ExpireOperatorSuggestionsArgs names the daily round that declines stale
routine suggestions; it takes no arguments.
*/
type ExpireOperatorSuggestionsArgs struct{}

/*
Kind names the job.
*/
func (ExpireOperatorSuggestionsArgs) Kind() string { return "expire_operator_suggestions" }

/*
ExpireOperatorSuggestions declines the stale routine suggestions.
*/
type ExpireOperatorSuggestions struct {
	Operator *operator.Engine
}

/*
Handle declines them.
*/
func (w *ExpireOperatorSuggestions) Handle(ctx context.Context, _ *queue.Job[ExpireOperatorSuggestionsArgs]) error {
	_, err := w.Operator.ExpireSuggestions(ctx)
	return err
}
