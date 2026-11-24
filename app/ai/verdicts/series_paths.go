/*
|--------------------------------------------------------------------------
| Series Paths
|--------------------------------------------------------------------------
|
| A series step can let the cheap model pick which path a person takes,
| from a few paths the team described, by reading a short profile of
| the person. The profile is untrusted and cut short, so it cannot
| steer the answer, and an answer outside the paths is no answer: the
| series sends the person down its fallback path. The instructions are
| verdicts/series_path.md, with the company's own instructions and
| paths filled in.
|
*/

package verdicts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
maxProfileRunes is the most of a profile the model reads.
*/
const maxProfileRunes = 4000

/*
Paths lets the model pick series paths.
*/
type Paths struct {
	Verdicts *Verdicts
}

/*
Pick returns the id of the path the model chose.
*/
func (p *Paths) Pick(ctx context.Context, in series.PickInput) (string, error) {
	ids := make([]string, 0, len(in.Paths))

	var b strings.Builder

	for _, path := range in.Paths {
		ids = append(ids, path.ID)
		fmt.Fprintf(&b, "- %s: %s. %s\n", path.ID, cut(path.Label, 80), cut(path.Description, 300))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"path"},
		"properties": map[string]any{"path": map[string]any{"type": "string", "enum": ids}},
	})
	if err != nil {
		return "", fmt.Errorf("write path schema: %w", err)
	}

	var answer struct {
		Path string `json:"path"`
	}

	err = p.Verdicts.ask(ctx, question{
		workspaceID: in.WorkspaceID, kind: "classify",
		system: prompts.Task(taskSeriesPath, prompts.Vars{
			"instructions": strings.TrimSpace(cut(in.Prompt, 1000)), "paths": strings.TrimSpace(b.String()),
		}),
		text: cut(in.Profile, maxProfileRunes), schemaName: "series_path", schema: schema,
	}, &answer)
	if err != nil {
		return "", err
	}

	for _, id := range ids {
		if id == answer.Path {
			return id, nil
		}
	}

	return "", nil
}
