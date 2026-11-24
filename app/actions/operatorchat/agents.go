/*
|--------------------------------------------------------------------------
| Agents Doing Operator's Work
|--------------------------------------------------------------------------
|
| A teammate's own agent, connected over the API or MCP, may do what
| Operator does: suggest changes for the team to review, keep a
| routine's state between runs and say a run found nothing. It acts as
| the teammate, and Operator's engine holds it to the rules its own
| runs follow. Doing a routine's work needs the right to manage the AI
| agent, as running a routine does.
|
*/

package operatorchat

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Agents serves agents that do Operator's work from outside it.

Rules:
  - Suggesting and listing suggestions need conversations:read, as
    talking with Operator does; a suggestion never applies on its own.
  - Reading a routine's state needs ai:read; suggesting for a routine,
    saving its state and reporting for it need ai_agent:manage.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[operator.ErrDisabled], [operator.ErrNoRoutine], a fault naming the
argument to fix, or a wrapped database error.
*/
type Agents struct {
	Operator *operator.Engine
}

/*
Suggest makes a suggestion of the kind for a teammate to review.
*/
func (a *Agents) Suggest(ctx context.Context, by operator.Outside, kind string, args json.RawMessage,
) (operator.Suggested, error) {
	if err := conversations.CanRead(by.Membership); err != nil {
		return operator.Suggested{}, err
	}
	if by.Routine != nil && !by.Membership.Can("ai_agent:manage") {
		return operator.Suggested{}, exceptions.ErrForbidden
	}
	return a.Operator.Suggest(ctx, by, kind, args)
}

/*
Recent lists the suggestions made lately that the teammate may see.
*/
func (a *Agents) Recent(ctx context.Context, by operator.Outside, days int, status string,
) ([]models.OperatorChangeset, error) {
	if err := conversations.CanRead(by.Membership); err != nil {
		return nil, err
	}
	return a.Operator.RecentSuggestions(ctx, by, days, status)
}

/*
State returns what a routine kept from its earlier runs.
*/
func (a *Agents) State(ctx context.Context, by operator.Outside) (json.RawMessage, error) {
	if !by.Membership.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}
	return a.Operator.RoutineState(ctx, by)
}

/*
SaveState saves what a routine's next run needs and returns the state
as it now stands.
*/
func (a *Agents) SaveState(ctx context.Context, by operator.Outside, args json.RawMessage) (json.RawMessage, error) {
	if !by.Membership.Can("ai_agent:manage") {
		return nil, exceptions.ErrForbidden
	}
	return a.Operator.SaveRoutineState(ctx, by, args)
}

/*
NothingToChange records that a run of the routine found nothing.
*/
func (a *Agents) NothingToChange(ctx context.Context, by operator.Outside, reason string) error {
	if !by.Membership.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}
	return a.Operator.NothingToChange(ctx, by, reason)
}

/*
SearchKnowledge searches the knowledge the AI agent answers from.
*/
func (a *Agents) SearchKnowledge(ctx context.Context, by operator.Outside, query string) ([]operator.Passage, error) {
	if err := conversations.CanRead(by.Membership); err != nil {
		return nil, err
	}
	return a.Operator.KnowledgePassages(ctx, by.Membership, query)
}
