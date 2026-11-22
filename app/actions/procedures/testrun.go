/*
|--------------------------------------------------------------------------
| Running A Saved Simulation
|--------------------------------------------------------------------------
|
| A saved simulation is replayed message by message against the real
| agent and the working copy, each turn handing its procedure state to
| the next, and what happened is then held against the simulation's
| checks. A check nobody could judge gives the run no verdict: it
| reports the error rather than pass or fail quietly.
|
*/

package procedures

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
simulated is one saved simulation to run: who runs it, the procedure,
the saved case, who watches, and what answers each turn: the
procedure's own Simulate when turn is nil.
*/
type simulated struct {
	m    authz.Membership
	id   ids.UUID
	c    models.ProcedureTestCase
	w    watch
	turn TurnFunc
}

/*
runTestCase plays a simulation: its first saved message as written, then
the simulated customer's answer to each reply, until the customer is
done, dimmi hands over, or the turns run out. What happened is then held
against the simulation's checks. A run where the customer could not be
played has no verdict: it reports the error, and can be run again.
*/
func (p *Procedures) runTestCase(ctx context.Context, run simulated) TestResult {
	res := TestResult{RanAt: p.Clock.Now(), Turns: []TestTurn{}, AssertionResults: []procedure.AssertionResult{}}
	script := scriptOf(run.c)
	if len(script.messages) == 0 {
		res.Error = "This simulation has no message to start from."
		return res
	}

	limit := min(len(script.messages)+extraTurns, maxTestMessages)
	chat := &played{observed: procedure.Observed{Branches: map[string]string{}}}
	next := customerMove{message: script.messages[0]}
	run.w.tell(ctx, runStarted, map[string]any{"turns": limit})
	for i := range limit {
		turn, err := p.playTurn(ctx, run, chat, next)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.Turns = append(res.Turns, turn)
		if chat.handedOff || i == limit-1 {
			break
		}
		var failure string
		if next, failure = p.nextMove(ctx, run, chat); failure != "" {
			res.Error = failure
			return res
		}
		if next.done {
			break
		}
	}

	run.w.tell(ctx, runJudging, nil)
	res.AssertionResults = procedure.Assert(script.assertions, chat.observed, transcript(chat.history),
		p.judge(ctx, run.m))
	res.Passed = procedure.Passed(res.AssertionResults)
	res.Error = unjudged(res.AssertionResults)

	return res
}

/*
played is a run's chat so far: what was said, the procedure's state,
what was observed, and whether dimmi handed over.
*/
type played struct {
	history   []pipeline.SimMessage
	state     *pipeline.SimState
	observed  procedure.Observed
	handedOff bool
}

/*
playTurn sends one customer move to dimmi and keeps what came back.
*/
func (p *Procedures) playTurn(ctx context.Context, run simulated, chat *played, move customerMove) (TestTurn, error) {
	msg := move.message
	turnAt := countTurns(chat.history)
	run.w.tell(ctx, runTurnStarted, map[string]any{"turn": turnAt, "message": msg.Text, "confirm": msg.Confirm})
	script := scriptOf(run.c)
	answer := run.turn
	if answer == nil {
		answer = p.Simulate
	}
	turn, err := answer(ctx, SimulateInput{
		Membership: run.m, ID: run.id, History: chat.history, Message: msg.Text, Persona: script.persona,
		Mocks: script.mocks, State: chat.state, Confirmed: confirmed(msg, chat.state),
		StreamID: run.w.turnStream(turnAt), Now: run.c.SimulatedAt,
	})
	if err != nil {
		return TestTurn{}, err
	}
	observe(&chat.observed, turn)
	out := TestTurn{
		Message: msg.Text, Confirm: msg.Confirm, Improvised: move.unknown, Reply: turn.Reply, Outcome: turn.Outcome,
		Calls: turn.Calls, ProcedureTrace: turn.ProcedureTrace, RunID: models.PublicID(models.PrefixAiRun, turn.RunID),
	}
	run.w.tell(ctx, runTurnFinished, map[string]any{"turn": turnAt, "result": out})
	chat.history = said(chat.history, msg.Text, turn.Reply)
	chat.state = &turn.State
	chat.handedOff = turn.Handoff != nil

	return out, nil
}

/*
countTurns is how many turns a chat has had: one per agent reply.
*/
func countTurns(history []pipeline.SimMessage) int {
	n := 0
	for _, m := range history {
		if m.From == "agent" {
			n++
		}
	}

	return n
}

/*
The reasons a run stops without a verdict because the customer could
not be played.
*/
const (
	noCustomer = "The simulated customer needs dimmi's built-in models, which this workspace does not have."
	silent     = "The simulated customer did not answer, so this run does not count. Run it again."
)

/*
extraTurns is how many turns past the saved chat's own a run may take,
for a procedure that now asks more than it did when the chat was saved.
*/
const extraTurns = 4

/*
customerMove is the customer's next turn: the message to send, whether
it is all the customer has left to do, and whether the customer had to
answer something the saved chat never gave.
*/
type customerMove struct {
	message TestMessage
	done    bool
	unknown bool
}

/*
nextMove asks the simulated customer what it does next. A press of
Confirm with no card waiting, or nothing left to say, ends the chat.
*/
func (p *Procedures) nextMove(ctx context.Context, run simulated, chat *played) (customerMove, string) {
	if p.Customer == nil {
		return customerMove{}, noCustomer
	}
	script := scriptOf(run.c)
	saved := make([]pipeline.SavedMessage, len(script.messages))
	for i, msg := range script.messages {
		saved[i] = pipeline.SavedMessage{Text: msg.Text, Confirm: msg.Confirm}
	}
	waiting := ""
	if chat.state != nil && len(chat.state.Pending) > 0 {
		waiting = chat.state.Pending[0].Summary
	}
	move, err := p.Customer.PlayCustomer(ctx, pipeline.CustomerTurn{
		WorkspaceID: run.m.Workspace.ID, MemberID: run.m.Member.ID, Name: script.persona.Name, Saved: saved,
		Run: chat.history, Confirming: waiting,
	})
	if errors.Is(err, pipeline.ErrCustomerUnavailable) {
		return customerMove{}, noCustomer
	}
	if err != nil {
		return customerMove{}, silent
	}
	presses := move.Confirm && waiting != ""
	if move.Done || (move.Message == "" && !presses) {
		return customerMove{done: true}, ""
	}

	return customerMove{message: TestMessage{Text: move.Message, Confirm: presses}, unknown: move.Unknown}, ""
}

/*
script is a saved simulation as it is replayed: who the customer is,
what they write, what the actions answer and what must hold.
*/
type script struct {
	persona    pipeline.SimPersona
	messages   []TestMessage
	mocks      map[string]actionkit.Mock
	assertions []procedure.Assertion
}

/*
scriptOf reads a saved simulation's script.
*/
func scriptOf(c models.ProcedureTestCase) script {
	out := script{mocks: map[string]actionkit.Mock{}}
	_ = json.Unmarshal(c.Persona, &out.persona)
	_ = json.Unmarshal(c.Messages, &out.messages)
	_ = json.Unmarshal(c.ActionMocks, &out.mocks)
	out.assertions, _ = procedure.ParseAssertions(c.Assertions)

	return out
}

/*
confirmed lists the waiting actions a scripted message confirms: all of
them when the message is a press of Confirm.
*/
func confirmed(msg TestMessage, state *pipeline.SimState) []string {
	if !msg.Confirm || state == nil {
		return nil
	}
	out := make([]string, 0, len(state.Pending))
	for _, pending := range state.Pending {
		out = append(out, pending.ActionID)
	}

	return out
}

/*
said adds one exchange to the conversation so far, leaving out a side
that wrote nothing, such as a press of Confirm.
*/
func said(history []pipeline.SimMessage, customer, agent string) []pipeline.SimMessage {
	if customer != "" {
		history = append(history, pipeline.SimMessage{From: "customer", Text: customer})
	}
	if agent != "" {
		history = append(history, pipeline.SimMessage{From: "agent", Text: agent})
	}

	return history
}

/*
unjudged is the error of a run with a check nobody could judge, which
has no verdict, or empty when every check was decided.
*/
func unjudged(results []procedure.AssertionResult) string {
	for _, r := range results {
		if r.Err != nil {
			return "A check could not be judged, so this run does not count: " + r.Err.Error() + ". Run it again."
		}
	}

	return ""
}

/*
observe folds one simulated turn into what the test saw.
*/
func observe(o *procedure.Observed, turn *pipeline.SimulationResult) {
	if turn.Matched && !slices.Contains(o.Procedures, turn.State.ProcedureID) {
		o.Procedures = append(o.Procedures, turn.State.ProcedureID)
	}

	o.Calls = append(o.Calls, turn.Calls...)
	if turn.Reply != "" {
		o.Replies = append(o.Replies, turn.Reply)
	}

	if h := turn.Handoff; h != nil {
		o.HandedOff, o.Team, o.Priority = true, h.TeamID, h.Priority
	}

	var trace struct {
		Steps []struct {
			StepID string `json:"step_id"`
			Branch string `json:"branch_taken"`
		} `json:"steps"`
		Guards []struct {
			Type          string `json:"type"`
			AttemptedTool string `json:"attempted_tool"`
		} `json:"guard_events"`
	}
	_ = json.Unmarshal(turn.ProcedureTrace, &trace)
	for _, s := range trace.Steps {
		if s.Branch == "then" || s.Branch == "else" {
			o.Branches[s.StepID] = s.Branch
		}
	}

	for _, g := range trace.Guards {
		if g.Type == "prerequisite_blocked" {
			o.Calls = append(o.Calls, procedure.ObservedCall{Tool: g.AttemptedTool, Blocked: true})
		}
	}
}

/*
judge is the rubric judge of a run, none when no judge is set up.
*/
func (p *Procedures) judge(ctx context.Context, m authz.Membership) procedure.Judge {
	if p.Judge == nil {
		return nil
	}

	return func(rubric, transcript string) (bool, string, error) {
		return p.Judge.JudgeRubric(ctx, pipeline.Rubric{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, Rubric: rubric, Transcript: transcript,
		})
	}
}

/*
transcript writes a simulated conversation for the judge.
*/
func transcript(history []pipeline.SimMessage) string {
	var b strings.Builder
	for _, msg := range history {
		who := "AGENT"
		if msg.From == "customer" {
			who = "CUSTOMER"
		}
		b.WriteString(who + ": " + msg.Text + "\n")
	}

	return b.String()
}

/*
workingCopyProblem says why the working copy cannot be simulated, or
nothing when it can: a copy with problems to fix before publishing would
only give misleading results, so its simulations are not run.
*/
func (p *Procedures) workingCopyProblem(ctx context.Context, m authz.Membership, id ids.UUID) (string, error) {
	var issues []procedure.Issue
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		current, err := find(ctx, models.New(tx), m.Workspace.ID, id)
		if err != nil {
			return err
		}
		d, err := underTest(current, "")
		if err != nil {
			return err
		}
		issues = procedure.Check(d)
		return nil
	})
	if err != nil || len(issues) == 0 {
		return "", err
	}
	more := ""
	if len(issues) > 1 {
		more = " (and " + strconv.Itoa(len(issues)-1) + " more)"
	}

	return "Not run: the working copy has problems to fix first. " + issues[0].Message + more, nil
}
