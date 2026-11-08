/*
|--------------------------------------------------------------------------
| Checking A Saved Audience
|--------------------------------------------------------------------------
|
| A saved audience must compile, and every saved audience it names must
| exist. Saved audiences may name each other two levels deep at most and
| never in a loop: deep nesting is hard to reason about, and a loop
| would never finish. The check looks at the whole workspace with the
| new rule in place, since a change here can make another audience's
| chain too long.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
maxNesting is how many saved audiences deep a chain may go below the
first.
*/
const maxNesting = 2

/*
check parses and compiles a rule, checks the saved audiences it names,
and returns it in stored form. No rule means everyone.
*/
func (a *Audiences) check(ctx context.Context, q *models.Queries, row models.Audience, raw json.RawMessage,
) (json.RawMessage, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{"visitors":"everyone"}`)
	}

	parsed, err := Parse(raw)
	if err != nil {
		return nil, err
	}

	others, err := q.ListAudiences(ctx, row.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list audiences: %w", err)
	}

	graph := map[string][]string{}
	for _, o := range others {
		rules, _ := audience.Parse(o.Rules)
		graph[models.PublicID(models.PrefixAudience, o.ID)] = audience.SavedIDs(rules.Rules)
	}

	self := models.PublicID(models.PrefixAudience, row.ID)
	graph[self] = audience.SavedIDs(parsed.Rules)

	if problems := nesting(graph, self); len(problems) > 0 {
		return nil, ErrInvalid.With("errors", problems)
	}

	stored, _ := json.Marshal(parsed)

	return stored, nil
}

/*
Parse reads and compiles a rule as a saved audience, every problem
listed in the error.
*/
func Parse(raw json.RawMessage) (audience.Audience, error) {
	parsed, err := audience.Parse(raw)
	if err == nil {
		_, err = audience.Compile(parsed, audience.Base(), audience.SavedAudience())
	}

	var found *audience.ProblemsError
	if err != nil && asProblems(err, &found) {
		return parsed, ErrInvalid.With("errors", found.Problems)
	}

	return parsed, err
}

/*
nesting finds missing saved audiences, loops, and chains deeper than
allowed, starting from the audience being saved and from every
audience that leads to it.
*/
func nesting(graph map[string][]string, self string) []audience.Problem {
	var found []audience.Problem
	for _, id := range graph[self] {
		if _, ok := graph[id]; !ok {
			found = append(found, audience.Problem{
				Code: audience.CodeMissingReference, Path: "rules", Field: "audience",
				Message: fmt.Sprintf("No saved audience has the id %s.", id),
			})
		}
	}

	for start := range graph {
		if start != self && !reaches(graph, start, self, map[string]bool{}) {
			continue
		}

		if p, ok := chain(graph, []string{start}); !ok {
			found = append(found, p)
			break
		}
	}

	return found
}

/*
reaches reports whether from leads to target.
*/
func reaches(graph map[string][]string, from, target string, seen map[string]bool) bool {
	if seen[from] {
		return false
	}

	seen[from] = true
	for _, next := range graph[from] {
		if next == target || reaches(graph, next, target, seen) {
			return true
		}
	}

	return false
}

/*
chain walks every path from the last audience of path, refusing a loop
or a path deeper than allowed.
*/
func chain(graph map[string][]string, path []string) (audience.Problem, bool) {
	last := path[len(path)-1]
	for _, next := range graph[last] {
		if slices.Contains(path, next) {
			return audience.Problem{
				Code: audience.CodeAudienceCycle, Path: "rules", Field: "audience",
				Message: "Saved audiences cannot name each other in a loop: " +
					strings.Join(append(path, next), " → ") + ".",
			}, false
		}

		if len(path) > maxNesting {
			return audience.Problem{
				Code: audience.CodeLimitExceeded, Path: "rules", Field: "audience",
				Message: "Saved audiences can name other saved audiences two levels deep at most.",
			}, false
		}

		if p, ok := chain(graph, append(path, next)); !ok {
			return p, false
		}
	}

	return audience.Problem{}, true
}

/*
asProblems reports whether err carries rule problems.
*/
func asProblems(err error, target **audience.ProblemsError) bool {
	return errors.As(err, target)
}
