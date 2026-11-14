/*
|--------------------------------------------------------------------------
| The Judge
|--------------------------------------------------------------------------
|
| One model grades every reply against the expert's answer, with the
| prompt in prompts/judge.md and a fixed response schema, at the model's
| default temperature, so a score means the same in every comparison.
|
*/

package costsim

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

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
	req := contracts.ChatRequest{
		Model: j.Model, System: []contracts.SystemBlock{{Text: judgePrompt, Cacheable: true}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: user}}},
		},
		MaxOutputTokens: 600, Effort: contracts.EffortLow,
	}
	if j.LLM.Capabilities(j.Model).JSONSchema {
		req.Schema = &contracts.ResponseSchema{Name: "grade", Schema: json.RawMessage(gradeSchema)}
	} else {
		req.MaxOutputTokens = 2000
		req.System = append(req.System, contracts.SystemBlock{Text: "Answer with only a JSON object matching " +
			"this schema, nothing before or after it: " + gradeSchema})
	}
	res, err := j.LLM.Chat(ctx, req)
	if err != nil {
		return Grade{}, err
	}
	var g Grade
	if err := json.Unmarshal([]byte(jsonObject(res.Text)), &g); err != nil {
		return Grade{}, fmt.Errorf("read verdict: %w", err)
	}
	g.Score = min(max(g.Score, 1), 5)
	return g, nil
}

/*
gradeSchema is the verdict's shape.
*/
const gradeSchema = `{"type":"object","properties":{"score":{"type":"integer","minimum":1,"maximum":5},` +
	`"correct":{"type":"boolean"},"contradicts":{"type":"boolean"},"reason":{"type":"string"}},` +
	`"required":["score","correct","contradicts","reason"],"additionalProperties":false}`

/*
jsonObject is the JSON object a reply holds, from its first opening
brace to its last closing one, for a model that wraps it in prose or a
code fence.
*/
func jsonObject(text string) string {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return text
	}
	return text[start : end+1]
}

/*
graded is one reply's verdict, or why it has none.
*/
type graded struct {
	grade Grade
	done  bool
	err   error
}

/*
gradeList is every reply's verdict, by conversation, and what judging
spent.
*/
type gradeList struct {
	items  []graded
	micros int64
}

/*
gradeAll grades the replies, several at a time when the run is
parallel.
*/
func (h *Harness) gradeAll(ctx context.Context, o Options, q QualityRun, cost *Report) gradeList {
	out := gradeList{items: make([]graded, len(cost.Conversations))}
	if o.DryRun || q.Judge.LLM == nil {
		return out
	}
	before, _, _ := h.Budget.Spent()
	slots := make(chan struct{}, max(o.Parallel, 1))
	var wg sync.WaitGroup
	for i, c := range cost.Conversations {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			g, err := q.Judge.grade(ctx, q.Items[i], c.Reply)
			out.items[i] = graded{grade: g, done: err == nil, err: err}
		})
	}
	wg.Wait()
	after, _, _ := h.Budget.Spent()
	out.micros = after - before
	return out
}

/*
Regrade grades a finished run's replies again with the judge, leaving
the answers as they were: twice-graded replies show how much the
judge's own scores move from run to run.
*/
func (h *Harness) Regrade(ctx context.Context, prior *QualityReport, q QualityRun) *QualityReport {
	out := *prior
	out.Items = slices.Clone(prior.Items)
	out.JudgeModel = q.Judge.Model
	out.Contradicting, out.Failed = 0, 0
	cost := &Report{Conversations: make([]ConversationResult, len(out.Items))}
	for i, it := range out.Items {
		cost.Conversations[i] = ConversationResult{Reply: it.Reply}
	}
	grades := h.gradeAll(ctx, Options{Parallel: 4}, q, cost)
	for i := range out.Items {
		if g := grades.items[i]; g.done {
			out.Items[i].Grade, out.Items[i].Graded = g.grade, true
		}
	}
	out.JudgeMicros = grades.micros
	out.summarize()
	return &out
}
