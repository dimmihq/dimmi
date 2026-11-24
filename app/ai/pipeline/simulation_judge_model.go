/*
|--------------------------------------------------------------------------
| Judging With A Model That Writes
|--------------------------------------------------------------------------
|
| A simulation's written checks are read by a model that can say why.
| It reads the whole transcript once and returns, for every check, a
| reason and whether the check was met, as typed JSON. The reason is
| what the team reads under a check that failed, so a verdict they
| disagree with is one they can argue with: reword the check, or see
| what the agent did.
|
| The judge is never the model that wrote the replies. Of the models
| the install serves, GPT-6 Luna judges an agent that answers on
| anything else, and Claude Haiku 5.5 judges an agent that answers on
| Luna. An install that serves neither, or a judge that fails twice,
| falls back to the decision model, one check at a time, which answers
| met or not met and gives no reason.
|
| What the judge is told is resources/prompts/tasks/simulation/judge.md.
| The call is recorded as a judge run and spends like any other.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
Judge limits: how many times the model is asked, the tokens its
verdicts may take, and a reason's length in runes.
*/
const (
	judgeModelTries  = 2
	judgeModelTokens = 3000
	reasonRunes      = 400
)

/*
Rubrics are the written checks of one simulation run to judge
together: the workspace and teammate who asked, the checks, the
transcript, and the models that wrote the agent's replies.
*/
type Rubrics struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	Rubrics     []string
	Transcript  string
	Wrote       []string
}

/*
verdictSchema is the typed answer the judge returns: one verdict per
check, the reason before the verdict.
*/
const verdictSchema = `{"type":"object","additionalProperties":false,"required":["verdicts"],"properties":` +
	`{"verdicts":{"type":"array","items":{"type":"object","additionalProperties":false,` +
	`"required":["requirement","reason","met"],"properties":{"requirement":{"type":"integer"},` +
	`"reason":{"type":"string"},"met":{"type":"boolean"}}}}}}`

/*
JudgeRubrics judges every written check of a simulation run in one
call to a model that did not write the replies, and returns a verdict
per check in the order given, each with the judge's reason. When no
such model is served, or it fails, each check is judged by the
decision model instead.
*/
func (r *Runner) JudgeRubrics(ctx context.Context, in Rubrics) ([]procedure.Judgment, error) {
	if len(in.Rubrics) == 0 {
		return nil, nil
	}
	r.prepare()
	if model := r.judgeModel(in.Wrote); model != "" {
		if out, ok := r.judgeWithWriter(ctx, in, model); ok {
			return out, nil
		}
	}
	out := make([]procedure.Judgment, len(in.Rubrics))
	for i, rubric := range in.Rubrics {
		v, err := r.JudgeRubric(ctx, Rubric{
			WorkspaceID: in.WorkspaceID, MemberID: in.MemberID, Rubric: rubric, Transcript: in.Transcript,
		})
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

/*
judgeModel is the served model that judges an agent whose replies the
given models wrote: the first of Luna and Haiku that wrote none of
them, or nothing when the install serves neither.
*/
func (r *Runner) judgeModel(wrote []string) string {
	served := r.LLM.ManagedModels()
	for _, id := range []string{"gpt-6-luna", "claude-haiku-5-5"} {
		if aimodels.Find(served, id) != nil && !slices.Contains(wrote, id) {
			return id
		}
	}
	return ""
}

/*
judgeWithWriter asks the model for its verdicts, twice at most, and
records the judge run. It reports false when the model could not be
reached or its answer could not be read.
*/
func (r *Runner) judgeWithWriter(ctx context.Context, in Rubrics, model string) ([]procedure.Judgment, bool) {
	st, key, err := r.openWriterJudge(ctx, in)
	if err != nil {
		return nil, false
	}
	llm, err := r.LLM.ForKey(ctx, key)
	if err != nil {
		return nil, false
	}
	var checks strings.Builder
	for i, rubric := range in.Rubrics {
		fmt.Fprintf(&checks, "%d. %s\n", i+1, rubric)
	}
	chat := contracts.ChatRequest{
		Model:  model,
		System: []contracts.SystemBlock{{Text: prompts.Task("simulation/judge", nil), Cacheable: true}},
		Messages: []contracts.ChatMessage{
			userMessage(
				"TRANSCRIPT:\n" + in.Transcript + "\n\nREQUIREMENTS:\n" + checks.String()),
		},
		MaxOutputTokens: judgeModelTokens, Effort: contracts.EffortLow,
		Schema: &contracts.ResponseSchema{Name: "simulation_verdicts", Schema: json.RawMessage(verdictSchema)},
	}
	var out []procedure.Judgment
	for range judgeModelTries {
		if out = r.askWriter(ctx, st, writerCall{llm: llm, key: key, chat: chat, checks: len(in.Rubrics)}); out != nil {
			break
		}
	}
	d := &decision{outcome: outcomeAnswered}
	if out == nil {
		d = failed("provider_error", ErrNoJudgment)
	}
	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		return r.recordSpend(ctx, tx, st, d)
	})
	return out, out != nil
}

/*
writerCall is one call to the judge: the driver and key it goes
through, the request, and how many checks it must answer.
*/
type writerCall struct {
	llm    contracts.LLM
	key    models.AiProviderKey
	chat   contracts.ChatRequest
	checks int
}

/*
askWriter makes the call, records it as a step of the judge run and
reads the verdicts, or returns nothing when the call failed or did not
answer every check.
*/
func (r *Runner) askWriter(ctx context.Context, st *run, call writerCall) []procedure.Judgment {
	ctx, cancel := r.timeout(ctx, r.Timeouts.GenerateTotal)
	defer cancel()
	started := r.Clock.Now()
	res, err := call.llm.Chat(ctx, call.chat)
	if res == nil {
		res = &contracts.ChatResponse{}
	}
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindVerify, llm: call.llm, key: &keyRef{id: call.key.ID, provider: call.key.Provider},
		model: call.chat.Model, started: started, request: transcriptOf(call.chat), response: res.Text,
		usage: res.Usage, err: err, requestID: res.RequestID, responseModel: res.Model,
	})
	if err != nil {
		return nil
	}
	return readVerdicts(res.Text, call.checks)
}

/*
readVerdicts reads the judge's typed answer into one judgment per
check, or nothing unless every check has exactly one verdict.
*/
func readVerdicts(text string, checks int) []procedure.Judgment {
	var parsed struct {
		Verdicts []struct {
			Requirement int    `json:"requirement"`
			Reason      string `json:"reason"`
			Met         bool   `json:"met"`
		} `json:"verdicts"`
	}
	if json.Unmarshal([]byte(text), &parsed) != nil || len(parsed.Verdicts) != checks {
		return nil
	}
	out := make([]procedure.Judgment, checks)
	seen := make([]bool, checks)
	for _, v := range parsed.Verdicts {
		i := v.Requirement - 1
		if i < 0 || i >= checks || seen[i] {
			return nil
		}
		seen[i] = true
		word := "Not met."
		if v.Met {
			word = "Met."
		}
		reason := truncate(strings.TrimSpace(v.Reason), reasonRunes)
		out[i] = procedure.Judgment{Passed: v.Met, Message: cmpOr(reason, word)}
	}
	return out
}

/*
openWriterJudge loads the workspace's agent and the key the served
models are reached through, and opens the judge run.
*/
func (r *Runner) openWriterJudge(ctx context.Context, in Rubrics) (*run, models.AiProviderKey, error) {
	var st *run
	var key models.AiProviderKey
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, _, err := r.loadSimulation(ctx, tx, SimulationInput{WorkspaceID: in.WorkspaceID})
		if err != nil {
			return err
		}
		if key, err = models.New(tx).FindManagedProviderKey(ctx, in.WorkspaceID); err != nil {
			return err
		}
		args := Args{WorkspaceID: in.WorkspaceID, Kind: judgeKind, MemberID: &in.MemberID}
		row, err := r.createRun(ctx, tx, e, args)
		if err != nil {
			return err
		}
		st = &run{env: e, rec: r.newRecorder(row), args: args, settings: settingsFor(e)}
		return nil
	})
	if err != nil {
		return nil, key, err
	}
	if st == nil {
		return nil, key, errors.New("pipeline: judge run not opened")
	}
	return st, key, nil
}
