/*
|--------------------------------------------------------------------------
| Operator Agent Resources
|--------------------------------------------------------------------------
|
| What an agent doing Operator's work reads back: what came of a
| suggestion, the suggestions made lately in a few words each, and a
| routine's saved state. Each is kept short, since an agent pays for
| every word it reads.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorSuggestionOutcome is what came of a suggestion. Made is false
when the same suggestion was made before; Message says what happened;
Suggestion is the changeset a teammate reviews, when one was made.
*/
type OperatorSuggestionOutcome struct {
	Object     string             `json:"object"`
	Made       bool               `json:"made"`
	Message    string             `json:"message"`
	Suggestion *OperatorChangeset `json:"suggestion"`
}

/*
NewOperatorSuggestionOutcome shapes what came of a suggestion.
*/
func NewOperatorSuggestionOutcome(s operator.Suggested) OperatorSuggestionOutcome {
	out := OperatorSuggestionOutcome{Object: "operator_suggestion_outcome", Message: s.Text}
	if s.Changeset != nil {
		out.Made, out.Suggestion = true, new(NewOperatorChangeset(*s.Changeset))
	}
	return out
}

/*
OperatorRecentSuggestion is one suggestion made lately, in the few
words an agent needs to not make it again: its status, the day it was
made, its title, the key that keeps it from being made twice, and each
change it holds as one line.
*/
type OperatorRecentSuggestion struct {
	ID              string   `json:"id"`
	Object          string   `json:"object"`
	Status          string   `json:"status"`
	SuggestedOn     string   `json:"suggested_on"`
	SuggestionTitle string   `json:"suggestion_title"`
	DedupKey        *string  `json:"dedup_key"`
	RoutineID       *string  `json:"routine_id"`
	Changes         []string `json:"changes"`
}

/*
NewOperatorRecentSuggestions shapes the suggestions made lately.
*/
func NewOperatorRecentSuggestions(rows []models.OperatorChangeset) []OperatorRecentSuggestion {
	out := make([]OperatorRecentSuggestion, len(rows))
	for i, c := range rows {
		out[i] = OperatorRecentSuggestion{
			ID: models.PublicID(models.PrefixOperatorChangeset, c.ID), Object: "operator_recent_suggestion",
			Status: c.Status, SuggestedOn: c.CreatedAt.UTC().Format("2006-01-02"), SuggestionTitle: c.Title,
			DedupKey: c.DedupKey, RoutineID: optionalID(models.PrefixOperatorRoutine, c.RoutineID),
			Changes: []string{},
		}
		for _, op := range operator.Operations(c.Operations) {
			out[i].Changes = append(out[i].Changes, op.Label)
		}
	}
	return out
}

/*
OperatorRoutineState is what a routine kept from its earlier runs.
Saved is false for a dry run, which keeps nothing.
*/
type OperatorRoutineState struct {
	Object    string          `json:"object"`
	RoutineID string          `json:"routine_id"`
	State     json.RawMessage `json:"state"`
	Saved     bool            `json:"saved"`
}

/*
OperatorNothingToChange says a run that found nothing was recorded.
Recorded is false for a dry run.
*/
type OperatorNothingToChange struct {
	Object    string `json:"object"`
	RoutineID string `json:"routine_id"`
	Recorded  bool   `json:"recorded"`
	Message   string `json:"message"`
}
