/*
|--------------------------------------------------------------------------
| Judgments
|--------------------------------------------------------------------------
|
| Some things the pipeline must know cannot be read off structure: what
| a reply says, what a step needs, whether a customer gave it. They are
| never guessed from wording, which breaks on the next phrasing and in
| every other language. A model reads the text and answers yes or no.
|
| The decision model answers first: it is fast, cheap and returns a
| probability per question. When it fails it is asked again, and when it
| still fails, or none is set up, the workspace's answer model is asked
| the same questions and held to one true or false each by a response
| schema. A caller gets nothing back only when no model answered, and
| then falls back on what code knows for certain.
|
| Every question is a file under resources/prompts/tasks/judgments, and
| what the answer model is told about judging is
| resources/prompts/tasks/judge/system.md.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
judged is the least probability at which a model's answer counts as
yes. Sure is the higher bar for a yes that costs the customer
something, such as holding their reply back.
*/
const (
	judged = 0.5
	sure   = 0.85
)

/*
decideTries is how many times the decision model is asked before the
answer model is asked in its place; judgeTokens bounds that model's
answer.
*/
const (
	decideTries = 2
	judgeTokens = 600
)

/*
decide asks the decision model, records the call as a step of the run
and returns its answers by question, or nothing when it failed.
*/
func (r *Runner) decide(
	ctx context.Context, st *run, req contracts.DecisionRequest,
) map[string]contracts.DecisionAnswer {
	started := r.Clock.Now()
	res, err := r.Decider.Decide(ctx, req)
	provider := r.DecisionProvider
	if provider == "" {
		provider = "decision"
	}
	rec := stepRecord{
		kind: enums.AIRunStepKindVerify, provider: provider, started: started, request: req.State, err: err,
	}
	if err != nil || res == nil {
		r.step(ctx, st, rec)
		return nil
	}
	nouls := make(map[string]float64, len(res.Answers))
	for name, a := range res.Answers {
		nouls[name] = a.Noul
	}
	answers, _ := json.Marshal(nouls)
	cost := int64(float64(res.Usage.InputTokens) * r.DecisionPrice)
	rec.model, rec.response, rec.cost = res.Model, string(answers), &cost
	rec.usage = contracts.LLMUsage{
		InputTokens: res.Usage.InputTokens, OutputTokens: res.Usage.OutputTokens, Reported: true,
	}
	r.step(ctx, st, rec)

	return res.Answers
}

/*
judge answers yes-or-no questions about one state, each with how likely
yes is. The decision model is asked, and asked again when it fails;
after that, or with none set up, the workspace's answer model is asked,
whose yes is a plain one or zero. It returns nothing when no model
answered.
*/
func (r *Runner) judge(ctx context.Context, st *run, req contracts.DecisionRequest) map[string]float64 {
	if r.Decider != nil {
		for range decideTries {
			answers := r.decide(ctx, st, req)
			if answers == nil {
				continue
			}
			out := make(map[string]float64, len(answers))
			for name, a := range answers {
				out[name] = a.Noul
			}
			return out
		}
		telemetry.Recovered(ctx, "decision_model_failed", "answer_model")
	}
	answers := r.judgeWithModel(ctx, st, req)
	if answers == nil && st.env.AnswerKey != nil {
		telemetry.Recovered(ctx, "judgment_failed", "no_answer")
	}
	return answers
}

/*
question reads one question for the decision model from its file under
resources/prompts/tasks/judgments, with its placeholders filled.
*/
func question(name string, vars prompts.Vars) contracts.DecisionQuestion {
	q := prompts.Judgment(name, vars)
	return contracts.DecisionQuestion{Type: contracts.DecisionNoul, Instructions: q.Ask, True: q.True, False: q.False}
}

/*
judgeAsk lays a state and its questions out for the answer model as
resources/prompts/tasks/judge/system.md describes them: the state in
one tag, then each question by name with what is asked and what makes
it true and false. Everything is escaped, so no text can close a tag.
*/
func judgeAsk(req contracts.DecisionRequest, names []string) string {
	var ask strings.Builder
	ask.WriteString("<state>\n" + escape(req.State) + "\n</state>\n<questions>")
	for _, name := range names {
		q := req.Questions[name]
		ask.WriteString("\n<question name=\"" + escape(name) + "\">" +
			"\n<ask>" + escape(q.Instructions) + "</ask>" +
			"\n<true_when>" + escape(q.True) + "</true_when>" +
			"\n<false_when>" + escape(q.False) + "</false_when>" +
			"\n</question>")
	}
	ask.WriteString("\n</questions>")
	return ask.String()
}

/*
judgeWithModel asks the workspace's answer model the questions and
reads its answers, which the response schema holds to one true or false
per question. The call is recorded as a step of the run.
*/
func (r *Runner) judgeWithModel(ctx context.Context, st *run, req contracts.DecisionRequest) map[string]float64 {
	e := st.env
	if e.AnswerKey == nil {
		return nil
	}
	llm, err := r.LLM.ForKey(ctx, *e.AnswerKey)
	if err != nil {
		telemetry.Recovered(ctx, "answer_model_unavailable", "no_answer")
		return nil
	}
	names := slices.Sorted(maps.Keys(req.Questions))
	props := make(map[string]any, len(names))
	for _, name := range names {
		props[name] = map[string]any{"type": "boolean"}
	}
	schema, _ := json.Marshal(map[string]any{
		"type": "object", "properties": props, "required": names, "additionalProperties": false,
	})
	model := strv(e.Agent.AnswerModel)
	chat := contracts.ChatRequest{
		Model: model, System: []contracts.SystemBlock{{Text: prompts.Task("judge/system", nil), Cacheable: true}},
		Messages: []contracts.ChatMessage{userMessage(judgeAsk(req, names))}, MaxOutputTokens: judgeTokens,
		Effort: contracts.EffortLow, Schema: &contracts.ResponseSchema{Name: "procedure_judgment", Schema: schema},
	}
	ctx, cancel := r.timeout(ctx, r.Timeouts.GenerateTotal)
	defer cancel()
	started := r.Clock.Now()
	res, err := llm.Chat(ctx, chat)
	if res == nil {
		res = &contracts.ChatResponse{}
	}
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindVerify, llm: llm, key: &keyRef{id: e.AnswerKey.ID, provider: e.AnswerKey.Provider},
		model: model, started: started, request: transcriptOf(chat), response: res.Text, usage: res.Usage, err: err,
		requestID: res.RequestID, responseModel: res.Model,
	})
	if err != nil {
		return nil
	}
	return plainAnswers(res.Text)
}

/*
plainAnswers reads the answer model's true or false per question as one
or zero, or nothing when its answer cannot be read.
*/
func plainAnswers(text string) map[string]float64 {
	var said map[string]bool
	if json.Unmarshal([]byte(text), &said) != nil {
		return nil
	}
	out := make(map[string]float64, len(said))
	for name, yes := range said {
		if yes {
			out[name] = 1
		} else {
			out[name] = 0
		}
	}
	return out
}
