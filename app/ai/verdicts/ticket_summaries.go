/*
|--------------------------------------------------------------------------
| Tracker Summary Verdict
|--------------------------------------------------------------------------
|
| The cheap model reads the first words of every customer linked to a
| tracker and says in a few sentences what they report: the symptom,
| where it happens and anything customers share in common. Customers'
| words are untrusted data inside <untrusted> and are cut short. The
| instructions are verdicts/tracker_summary.md.
|
*/

package verdicts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
Summarize writes what the tracker's customers report.
*/
func (t *Tickets) Summarize(ctx context.Context, q tickets.SummaryQuestion) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "Problem: %s\n", cut(q.Title, 200))

	for i, r := range q.Reports {
		fmt.Fprintf(&b, "Report %d: %s\n", i+1, cut(r, 400))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"summary"},
		"properties": map[string]any{"summary": map[string]any{"type": "string"}},
	})
	if err != nil {
		return "", fmt.Errorf("write summary schema: %w", err)
	}

	var answer struct {
		Summary string `json:"summary"`
	}

	err = t.Verdicts.ask(ctx, question{
		workspaceID: q.WorkspaceID, kind: "ticket_summary", system: prompts.Task(taskTrackerSummary, nil),
		text: cut(b.String(), maxProfileRunes*2), schemaName: "tracker_summary", schema: schema,
	}, &answer)

	return answer.Summary, err
}
