/*
|--------------------------------------------------------------------------
| Workflow Categories
|--------------------------------------------------------------------------
|
| A workflow's classify step lets the cheap model sort a conversation
| into one of the team's categories by reading what the customer wrote.
| The customer's words are untrusted and cut short, so they cannot
| steer the answer; the model must answer with one of the categories'
| ids, or none, and says how sure it is. An answer outside the
| categories is no answer, and the workflow takes its none path. The
| instructions are verdicts/workflow_category.md, with the team's own
| question and categories filled in.
|
*/

package verdicts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/app/workflows"
)

/*
Categories lets the model sort conversations for workflows.
*/
type Categories struct {
	Verdicts *Verdicts
}

/*
Classify returns the category the model chose.
*/
func (c *Categories) Classify(ctx context.Context, q workflows.ClassifyQuestion) (workflows.ClassifyAnswer, error) {
	choices := []string{"none"}

	var b strings.Builder

	for _, category := range q.Categories {
		choices = append(choices, category.ID)
		fmt.Fprintf(&b, "- %s: %s. %s\n", category.ID, cut(category.Label, 80), cut(category.Description, 300))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"category", "confidence"},
		"properties": map[string]any{
			"category":   map[string]any{"type": "string", "enum": choices},
			"confidence": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
		},
	})
	if err != nil {
		return workflows.ClassifyAnswer{}, fmt.Errorf("write category schema: %w", err)
	}

	var answer struct {
		Category   string `json:"category"`
		Confidence string `json:"confidence"`
	}

	conversation := q.ConversationID
	err = c.Verdicts.ask(ctx, question{
		workspaceID: q.WorkspaceID, kind: "classify", conversationID: &conversation,
		system: prompts.Task(taskWorkflowCategory, prompts.Vars{
			"instructions": strings.TrimSpace(cut(q.Instructions, 1000)), "categories": strings.TrimSpace(b.String()),
		}),
		text: cut(q.Text, maxProfileRunes), schemaName: "workflow_category", schema: schema,
	}, &answer)
	if err != nil {
		return workflows.ClassifyAnswer{}, err
	}

	if answer.Category == "none" {
		answer.Category = ""
	}

	return workflows.ClassifyAnswer{CategoryID: answer.Category, Confidence: answer.Confidence}, nil
}
