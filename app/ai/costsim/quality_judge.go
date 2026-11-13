/*
|--------------------------------------------------------------------------
| The Judge
|--------------------------------------------------------------------------
|
| One model grades every reply against the expert's answer, with the
| prompt in prompts/judge.md and a fixed response schema, at temperature
| zero, so a score means the same thing in every comparison.
|
*/

package costsim

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
judgePrompt is the judge's instructions.
*/
//go:embed prompts/judge.md
var judgePrompt string

/*
Judge grades replies with one model.
*/
type Judge struct {
	LLM   contracts.LLM
	Model string
}

/*
grade asks the judge for its verdict.
*/
func (j Judge) grade(ctx context.Context, it QualityItem, reply string) (Grade, error) {
	if strings.TrimSpace(reply) == "" {
		return Grade{Score: 1, Reason: "no reply"}, nil
	}
	user := "<question>\n" + it.Question + "\n</question>\n\n<expert_answer>\n" + it.Expected +
		"\n</expert_answer>\n\n<reply>\n" + reply + "\n</reply>"
	res, err := j.LLM.Chat(ctx, contracts.ChatRequest{
		Model: j.Model, System: []contracts.SystemBlock{{Text: judgePrompt, Cacheable: true}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: user}}},
		},
		MaxOutputTokens: 400, Effort: contracts.EffortLow, Temperature: new(0.0),
		Schema: &contracts.ResponseSchema{Name: "grade", Schema: json.RawMessage(`{"type":"object",` +
			`"properties":{"score":{"type":"integer","minimum":1,"maximum":5},"correct":{"type":"boolean"},` +
			`"contradicts":{"type":"boolean"},"reason":{"type":"string"}},` +
			`"required":["score","correct","contradicts","reason"],"additionalProperties":false}`)},
	})
	if err != nil {
		return Grade{}, err
	}
	var g Grade
	if err := json.Unmarshal([]byte(res.Text), &g); err != nil {
		return Grade{}, fmt.Errorf("read verdict: %w", err)
	}
	g.Score = min(max(g.Score, 1), 5)
	return g, nil
}
