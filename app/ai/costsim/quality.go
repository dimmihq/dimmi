/*
|--------------------------------------------------------------------------
| Answer Quality
|--------------------------------------------------------------------------
|
| A saving that makes answers worse is not a saving. The quality mode
| asks each question of a public set as a new customer conversation,
| then grades the reply against the expert's answer with one fixed judge
| model and one fixed prompt, kept in prompts/judge.md, so scores stay
| comparable from run to run. Per question it records whether search
| kept any article the expert answer came from, the judge's score and
| reason, whether the reply contradicts the expert, and what it cost.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
KindQuality marks a quality question's conversation.
*/
const KindQuality = "quality"

/*
QualityItem is one question to ask: its key, the question, the
expert's answer and the help center slugs of the articles holding it.
*/
type QualityItem struct {
	Key      string
	Question string
	Expected string
	Slugs    []string
}

/*
Grade is the judge's verdict on one reply.
*/
type Grade struct {
	Score       int    `json:"score"`
	Correct     bool   `json:"correct"`
	Contradicts bool   `json:"contradicts"`
	Reason      string `json:"reason"`
}

/*
QualityResult is one graded question.
*/
type QualityResult struct {
	Key        string              `json:"key"`
	Question   string              `json:"question"`
	Reply      string              `json:"reply"`
	Hit        bool                `json:"retrieval_hit"`
	Grade      Grade               `json:"grade"`
	Graded     bool                `json:"graded"`
	Failed     bool                `json:"failed"`
	Outcome    string              `json:"outcome"`
	ModelCalls int                 `json:"model_calls"`
	CostMicros int64               `json:"cost_micros"`
	Ledger     map[string]PartCost `json:"ledger,omitempty"`
	Error      string              `json:"error,omitempty"`
}

/*
QualityReport is a whole quality run.
*/
type QualityReport struct {
	Set           string          `json:"set"`
	AgentModel    string          `json:"agent_model"`
	JudgeModel    string          `json:"judge_model"`
	Items         []QualityResult `json:"items"`
	HitRate       float64         `json:"retrieval_hit_rate"`
	AvgScore      float64         `json:"average_score"`
	CorrectShare  float64         `json:"correct_share"`
	Contradicting int             `json:"contradicting"`
	Failed        int             `json:"failed"`
	AnswerMicros  float64         `json:"cost_per_answer_micros"`
	JudgeMicros   int64           `json:"judge_micros"`
	Cost          *Report         `json:"cost"`
}

/*
QualityRun is what a quality run asks: the set's name, its questions
and the judge.
*/
type QualityRun struct {
	Set   string
	Items []QualityItem
	Judge Judge
}

/*
RunQuality asks every question as a new conversation and grades the
replies.
*/
func (h *Harness) RunQuality(ctx context.Context, o Options, q QualityRun) (*QualityReport, error) {
	set, items, judge := q.Set, q.Items, q.Judge
	o.Scenarios = make([]Scenario, 0, len(items))
	for _, it := range items {
		o.Scenarios = append(o.Scenarios, Scenario{
			Key: it.Key, Kind: KindQuality, Subject: "Question", Messages: []string{it.Question},
		})
	}
	o.Conversations, o.MaxTurns = len(items), 1
	cost, err := h.Run(ctx, o)
	if err != nil {
		return nil, err
	}
	grades := h.gradeAll(ctx, o, q, cost)
	out := &QualityReport{Set: set, AgentModel: cost.AgentModel, JudgeModel: judge.Model, Cost: cost}
	for i, c := range cost.Conversations {
		it := items[i]
		r := QualityResult{
			Key: it.Key, Question: it.Question, Reply: c.Reply, Outcome: c.Outcome, ModelCalls: c.ModelCalls,
			CostMicros: c.CostMicros, Ledger: c.Ledger, Error: c.Error, Hit: hit(c.Retrieved, it.Slugs),
			Failed: c.RunErrors > 0,
		}
		if g := grades.items[i]; g.err != nil {
			r.Error = strings.TrimSpace(r.Error + " judge: " + g.err.Error())
		} else if g.done {
			r.Grade, r.Graded = g.grade, true
		}
		out.Items = append(out.Items, r)
		o.Log("%3d %-10s hit %-5v score %d %s", i+1, it.Key, r.Hit, r.Grade.Score, r.Grade.Reason)
	}
	out.JudgeMicros = grades.micros
	out.summarize()
	return out, nil
}

/*
hit reports whether search kept a passage of any expected article.
*/
func hit(retrieved, slugs []string) bool {
	for _, url := range retrieved {
		for _, s := range slugs {
			if strings.HasSuffix(strings.TrimRight(url, "/"), "/"+s) {
				return true
			}
		}
	}
	return false
}

/*
summarize works out the run's figures.
*/
func (q *QualityReport) summarize() {
	var hits, graded, correct, n int
	var score float64
	var cost int64
	for _, it := range q.Items {
		if it.Failed {
			q.Failed++
			continue
		}
		n++
		if it.Hit {
			hits++
		}
		if it.Graded {
			graded++
			score += float64(it.Grade.Score)
			if it.Grade.Correct {
				correct++
			}
			if it.Grade.Contradicts {
				q.Contradicting++
			}
		}
		cost += it.CostMicros
	}
	if n > 0 {
		q.HitRate = float64(hits) / float64(n)
		q.AnswerMicros = float64(cost) / float64(n)
	}
	if graded > 0 {
		q.AvgScore = score / float64(graded)
		q.CorrectShare = float64(correct) / float64(graded)
	}
}

/*
Markdown is the run's summary for people.
*/
func (q *QualityReport) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Answer quality: WixQA %s\n\n", q.Set)
	fmt.Fprintf(&b, "- %d questions, agent model %s, judge %s\n", len(q.Items), q.AgentModel, q.JudgeModel)
	fmt.Fprintf(&b, "- retrieval hit rate %.1f%%, average score %.2f, judged correct %.1f%%, contradicting %d\n",
		q.HitRate*100, q.AvgScore, q.CorrectShare*100, q.Contradicting)
	if q.Failed > 0 {
		fmt.Fprintf(&b, "- %d questions failed on the provider and are left out of every figure\n", q.Failed)
	}
	fmt.Fprintf(&b, "- cost per answer %s; judging %s in all\n", cents(q.AnswerMicros),
		cents(float64(q.JudgeMicros)))
	b.WriteString("\nDataset: WixQA, Wix.com AI Research (MIT license).\n")
	if q.Cost != nil {
		b.WriteString(q.Cost.ledgerMarkdown())
	}
	b.WriteString("\n## Questions\n\n| key | hit | score | correct | contradicts | calls | cost | reason |\n" +
		strings.Repeat("|---", 8) + "|\n")
	items := slices.Clone(q.Items)
	for _, it := range items {
		fmt.Fprintf(&b, "| %s | %v | %d | %v | %v | %d | %s | %s |\n", it.Key, it.Hit, it.Grade.Score,
			it.Grade.Correct, it.Grade.Contradicts, it.ModelCalls, cents(float64(it.CostMicros)),
			strings.ReplaceAll(it.Grade.Reason, "|", "/"))
	}
	return b.String()
}

/*
Write saves the run as JSON and as a summary under the directory.
*/
func (q *QualityReport) Write(ctx context.Context, disk filesystem.Filesystem, dir string) (string, string, error) {
	raw, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return "", "", err
	}
	js, md := path.Join(dir, "quality.json"), path.Join(dir, "quality.md")
	if err := disk.Put(ctx, js, raw, filesystem.PutOptions{ContentType: "application/json"}); err != nil {
		return "", "", fmt.Errorf("write quality report: %w", err)
	}
	if err := disk.Put(ctx, md, []byte(q.Markdown()), filesystem.PutOptions{ContentType: "text/markdown"}); err != nil {
		return "", "", fmt.Errorf("write quality summary: %w", err)
	}
	return js, md, nil
}
