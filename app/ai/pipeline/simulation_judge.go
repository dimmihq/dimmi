/*
|--------------------------------------------------------------------------
| Judging A Rubric
|--------------------------------------------------------------------------
|
| A saved test may check something only words can say, such as "explains
| the 30-day policy politely". The decision model reads the simulated
| transcript and the rubric and says how likely the conversation meets
| it, so the agent never grades its own work. When the decision model
| fails it is asked once more; when it fails again the check has no
| verdict and the run reports an error, since an answer model grading
| its own kind of work is no judge. What the judge is asked is
| resources/prompts/tasks/judgments/simulation_check.md. The call is
| recorded as a judge run and spends like any other.
|
*/

package pipeline

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
judgeKind is the run kind of a rubric judged for a saved test.
*/
const judgeKind = "judge"

/*
clearlyMet is how likely the judge must find a check met for it to
pass. Below it and down to an even chance the judge is unsure: the
check neither passes nor fails, and the team is told to look. Under an
even chance the check is not met.
*/
const clearlyMet = 0.6

/*
unsureMessage is what the team reads under a check the judge could not
tell either way.
*/
const unsureMessage = "The judge could not tell. Read the reply, or make the check more specific."

/*
Rubric is a rubric to judge: the workspace and teammate who asked, the
rubric, and the simulated conversation's transcript.
*/
type Rubric struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	Rubric      string
	Transcript  string
}

/*
JudgeRubric asks whether a simulated conversation meets a rubric. The
decision model reads the transcript and answers how likely it is; when
it fails it is asked once more, and then ErrNoJudgment is returned. The
call is recorded as a judge run. A check passes when the judge finds it
clearly met and fails when it finds it not met; in between the judge is
unsure, and the check does neither. The verdict says that and nothing
more: how likely the judge found it is its own working, kept on the
run and not shown to the team.
*/
func (r *Runner) JudgeRubric(ctx context.Context, in Rubric) (procedure.Judgment, error) {
	r.prepare()
	st, err := r.openJudge(ctx, in)
	if err != nil {
		return procedure.Judgment{}, err
	}
	q := prompts.Judgment("simulation_check", prompts.Vars{"transcript": in.Transcript, "check": in.Rubric})
	answers := r.decideTwice(ctx, st, contracts.DecisionRequest{
		State: q.State,
		Questions: map[string]contracts.DecisionQuestion{"met": {
			Type: contracts.DecisionNoul, Instructions: q.Ask, True: q.True, False: q.False,
		}},
	})
	met, ok := answers["met"]
	d := &decision{outcome: outcomeAnswered}
	if !ok {
		err = ErrNoJudgment
		d = failed("provider_error", err)
	}
	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		return r.recordSpend(ctx, tx, st, d)
	})
	if err != nil {
		return procedure.Judgment{}, err
	}
	switch {
	case met >= clearlyMet:
		return procedure.Judgment{Passed: true, Message: "Met."}, nil
	case met >= judged:
		return procedure.Judgment{Unsure: true, Message: unsureMessage}, nil
	}
	return procedure.Judgment{Message: "Not met."}, nil
}

/*
ErrNoJudgment is returned when no model answered a judgment.
*/
var ErrNoJudgment = errors.New("the decision model did not answer")

/*
decideTwice asks the decision model, and asks again when it fails. It
returns nothing when neither answer came, or when no decision model is
set up.
*/
func (r *Runner) decideTwice(ctx context.Context, st *run, req contracts.DecisionRequest) map[string]float64 {
	if r.Decider == nil {
		return nil
	}
	for range decideTries {
		if answers := r.decide(ctx, st, req); answers != nil {
			out := make(map[string]float64, len(answers))
			for name, a := range answers {
				out[name] = a.Noul
			}
			return out
		}
	}
	return nil
}

/*
openJudge loads the workspace's agent and opens the judge run.
*/
func (r *Runner) openJudge(ctx context.Context, in Rubric) (*run, error) {
	var st *run
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, _, err := r.loadSimulation(ctx, tx, SimulationInput{WorkspaceID: in.WorkspaceID})
		if err != nil {
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
		return nil, err
	}
	if st.env.AnswerKey == nil {
		return nil, ErrNoAgent
	}
	return st, nil
}
