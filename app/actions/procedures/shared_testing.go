/*
|--------------------------------------------------------------------------
| Shared Simulation Engine
|--------------------------------------------------------------------------
|
| Agents are simulated the way procedures are: the same scripted and
| AI-played customer, the same mocks, the same checks and the same live
| progress. What differs is what answers each turn. These let an agent's
| simulations run on the engine procedures use, with their own answer.
|
*/

package procedures

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TurnFunc answers one simulated turn.
*/
type TurnFunc func(ctx context.Context, in SimulateInput) (*pipeline.SimulationResult, error)

/*
BuildTestCase applies a saved simulation's changes to its current shape
and checks them the way a procedure's are.
*/
func BuildTestCase(current models.ProcedureTestCase, in TestCaseInput) (models.ProcedureTestCase, error) {
	return testRow(current, in)
}

/*
CaseRun is one saved simulation to play: who plays it, the case, how
whoever watches follows it and the public id they know it by, and what
answers each turn.
*/
type CaseRun struct {
	Membership authz.Membership
	Case       models.ProcedureTestCase
	PublicID   string
	Run        TestRun
	Turn       TurnFunc
}

/*
PlayCase plays one saved simulation with the engine procedures use,
each turn answered by the run's own TurnFunc.
*/
func (p *Procedures) PlayCase(ctx context.Context, c CaseRun) TestResult {
	w := p.watching(c.Membership, c.Run, c.Case)
	if c.PublicID != "" {
		w.test = c.PublicID
	}
	return p.runTestCase(ctx, simulated{m: c.Membership, c: c.Case, w: w, turn: c.Turn})
}

/*
ReadConversation reads a real conversation's messages as a
simulation replays them, and splits off the customer's last message.
*/
func (p *Procedures) ReadConversation(ctx context.Context, m authz.Membership, conversation ids.UUID,
) ([]pipeline.SimMessage, error) {
	var out []pipeline.SimMessage
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = p.conversationScript(ctx, models.New(tx), procedureRef{ws: m.Workspace.ID, id: conversation})
		return err
	})
	return out, err
}

/*
SplitReplay splits a conversation into the history before the customer's
last message and that message; a message given takes its place.
*/
func SplitReplay(script []pipeline.SimMessage, message string) ([]pipeline.SimMessage, string) {
	return replay(script, message)
}
