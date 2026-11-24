/*
|--------------------------------------------------------------------------
| Operator Agent Controller
|--------------------------------------------------------------------------
|
| An agent a teammate connected does Operator's work here: it suggests
| changes for the team to review, lists what was suggested lately, reads
| and saves a routine's state, says a run found nothing, and reads the
| skills Operator's own runs read. The engine holds it to Operator's
| rules; this maps ids and shapes.
|
*/

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorAgentController serves agents doing Operator's work.
*/
type OperatorAgentController struct {
	Agents *operatorchat.Agents
	IDs    *ids.Registry
}

/*
SuggestArticleEdit suggests a fix to one article.
*/
func (c *OperatorAgentController) SuggestArticleEdit(ctx context.Context,
	in *requests.StoreArticleEditSuggestionRequest,
) (*OperatorSuggestionOutcomeOutput, error) {
	return c.suggest(ctx, operator.SuggestArticleEdit, in.Body, target{in.Body.RoutineID, in.Body.DryRun})
}

/*
SuggestNewArticle suggests a new article.
*/
func (c *OperatorAgentController) SuggestNewArticle(ctx context.Context,
	in *requests.StoreNewArticleSuggestionRequest,
) (*OperatorSuggestionOutcomeOutput, error) {
	return c.suggest(ctx, operator.SuggestNewArticle, in.Body, target{in.Body.RoutineID, in.Body.DryRun})
}

/*
SuggestProcedureEdit suggests an edit to one procedure.
*/
func (c *OperatorAgentController) SuggestProcedureEdit(ctx context.Context,
	in *requests.StoreProcedureEditSuggestionRequest,
) (*OperatorSuggestionOutcomeOutput, error) {
	return c.suggest(ctx, operator.SuggestProcedureEdit, in.Body, target{in.Body.RoutineID, in.Body.DryRun})
}

/*
SuggestChanges suggests changes to conversations and customers.
*/
func (c *OperatorAgentController) SuggestChanges(ctx context.Context, in *requests.StoreChangesSuggestionRequest,
) (*OperatorSuggestionOutcomeOutput, error) {
	return c.suggest(ctx, operator.SuggestChanges, in.Body, target{in.Body.RoutineID, in.Body.DryRun})
}

/*
target is the routine a call is for, by public id, and whether the
call is a dry run of it.
*/
type target struct {
	routine string
	dryRun  bool
}

/*
suggest makes a suggestion of the kind from the request's body.
*/
func (c *OperatorAgentController) suggest(ctx context.Context, kind string, body any, to target,
) (*OperatorSuggestionOutcomeOutput, error) {
	by, err := c.outside(ctx, to)
	if err != nil {
		return nil, err
	}
	if by.DryRun && by.Routine == nil {
		return nil, operator.ErrNoRoutine
	}
	args, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode suggestion: %w", err)
	}
	made, err := c.Agents.Suggest(ctx, by, kind, args)
	if err != nil {
		return nil, err
	}
	return &OperatorSuggestionOutcomeOutput{Body: fhttp.NewResource(resources.NewOperatorSuggestionOutcome(made))}, nil
}

/*
Recent lists the suggestions made lately, with their dedup keys.
*/
func (c *OperatorAgentController) Recent(ctx context.Context, in *requests.RecentOperatorSuggestionsRequest,
) (*OperatorRecentSuggestionsOutput, error) {
	by, err := c.outside(ctx, target{routine: in.RoutineID})
	if err != nil {
		return nil, err
	}
	rows, err := c.Agents.Recent(ctx, by, in.DaysBack, in.Status)
	if err != nil {
		return nil, err
	}
	items := resources.NewOperatorRecentSuggestions(rows)
	page := pagination.Page[resources.OperatorRecentSuggestion]{Items: items, Limit: len(items)}
	return &OperatorRecentSuggestionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
State reads what a routine kept from its earlier runs.
*/
func (c *OperatorAgentController) State(ctx context.Context, in *requests.OperatorRoutineStateRequest,
) (*OperatorRoutineStateOutput, error) {
	by, err := c.outside(ctx, target{routine: in.Routine})
	if err != nil {
		return nil, err
	}
	state, err := c.Agents.State(ctx, by)
	if err != nil {
		return nil, err
	}
	return stateOutput(in.Routine, state, true), nil
}

/*
SaveState saves what a routine's next run needs.
*/
func (c *OperatorAgentController) SaveState(ctx context.Context, in *requests.SaveOperatorRoutineStateRequest,
) (*OperatorRoutineStateOutput, error) {
	by, err := c.outside(ctx, target{in.Routine, in.Body.DryRun})
	if err != nil {
		return nil, err
	}
	args, err := json.Marshal(in.Body)
	if err != nil {
		return nil, fmt.Errorf("encode state: %w", err)
	}
	state, err := c.Agents.SaveState(ctx, by, args)
	if err != nil {
		return nil, err
	}
	return stateOutput(in.Routine, state, !in.Body.DryRun), nil
}

/*
stateOutput shapes a routine's state.
*/
func stateOutput(routine string, state json.RawMessage, saved bool) *OperatorRoutineStateOutput {
	return &OperatorRoutineStateOutput{Body: fhttp.NewResource(resources.OperatorRoutineState{
		Object: "operator_routine_state", RoutineID: routine, State: state, Saved: saved,
	})}
}

/*
NothingToChange records that a run of the routine found nothing.
*/
func (c *OperatorAgentController) NothingToChange(ctx context.Context, in *requests.StoreNothingToChangeRequest,
) (*OperatorNothingToChangeOutput, error) {
	by, err := c.outside(ctx, target{in.Routine, in.Body.DryRun})
	if err != nil {
		return nil, err
	}
	if err := c.Agents.NothingToChange(ctx, by, in.Body.Reason); err != nil {
		return nil, err
	}
	out := resources.OperatorNothingToChange{
		Object: "operator_nothing_to_change", RoutineID: in.Routine, Recorded: !in.Body.DryRun,
		Message: "Noted: nothing needs the team's attention, so nobody is notified.",
	}
	if in.Body.DryRun {
		out.Message = "Noted for this dry run only: a dry run records nothing, which is expected."
	}
	return &OperatorNothingToChangeOutput{Body: fhttp.NewResource(out)}, nil
}

/*
SearchKnowledge searches the knowledge the AI agent answers from.
*/
func (c *OperatorAgentController) SearchKnowledge(ctx context.Context, in *requests.SearchOperatorKnowledgeRequest,
) (*OperatorKnowledgePassagesOutput, error) {
	found, err := c.Agents.SearchKnowledge(ctx, operator.Outside{Membership: membership(ctx)}, in.SearchText)
	if err != nil {
		return nil, err
	}
	items := resources.NewOperatorKnowledgePassages(found)
	page := pagination.Page[resources.OperatorKnowledgePassage]{Items: items, Limit: len(items)}
	return &OperatorKnowledgePassagesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Skills lists the skills, without their steps.
*/
func (c *OperatorAgentController) Skills(_ context.Context, _ *requests.NoInput) (*OperatorSkillsOutput, error) {
	items := resources.NewOperatorSkills(operator.AllSkills())
	page := pagination.Page[resources.OperatorSkill]{Items: items, Limit: len(items)}
	return &OperatorSkillsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Skill reads one skill's steps; an unknown name is answered with the
names there are.
*/
func (c *OperatorAgentController) Skill(_ context.Context, in *requests.OperatorSkillRequest,
) (*OperatorSkillOutput, error) {
	var names []string
	for _, s := range operator.AllSkills() {
		if s.Name == in.Skill {
			return &OperatorSkillOutput{Body: fhttp.NewResource(resources.NewOperatorSkill(s))}, nil
		}
		names = append(names, s.Name)
	}
	return nil, fault.New(fault.NotFound, "skill_not_found", fmt.Sprintf("There is no skill %q. The skills are %s.",
		in.Skill, strings.Join(names, ", ")))
}

/*
outside is who the call acts as: the caller, for the routine named when
one is.
*/
func (c *OperatorAgentController) outside(ctx context.Context, to target) (operator.Outside, error) {
	by := operator.Outside{Membership: membership(ctx), DryRun: to.dryRun}
	if to.routine == "" {
		return by, nil
	}
	id, err := c.IDs.Decode(string(models.PrefixOperatorRoutine), to.routine)
	if err != nil {
		return by, exceptions.ErrNotFound
	}
	by.Routine = &id
	return by, nil
}
