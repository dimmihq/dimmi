/*
|--------------------------------------------------------------------------
| dimmi Inside A Workflow Simulation
|--------------------------------------------------------------------------
|
| A step that hands the conversation to an agent plays the agent's turns
| on the agent simulator: as the agent the step names, or else the one
| placed where the simulation's source conversation came in, or else the
| workspace's default, with the chat so far as history. The customer
| answers each reply, and the step ends the way a real one would: dimmi
| hands over, the customer is done (the resolved path), or the customer
| says nothing more (the quiet path). A turn that transfers the chat
| goes on as the agent it went to, as in a real conversation.
|
*/

package workflowsims

import (
	"context"
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
Handle plays dimmi's turns in an agent step, the customer answering
each, and says how they ended.
*/
func (c *chat) Handle(ctx context.Context, h engine.DryHandle) (engine.DryHandled, error) {
	if c.s.Simulator == nil {
		return handedOver("agent_disabled"), nil
	}
	turn := agentAsk{stepID: h.StepID, agent: c.stepAgent(ctx, h.AgentID)}
	for range maxAgentTurns {
		if c.waiting == "" && !turn.confirm {
			ended, confirm, err := c.answerAgent(ctx, h.StepID)
			if err != nil || ended != nil {
				return deref(ended), err
			}
			turn.confirm = confirm
		}
		res, err := c.agentTurn(ctx, turn)
		if errors.Is(err, pipeline.ErrNoAgent) {
			return handedOver("agent_disabled"), nil
		}
		if err != nil {
			return engine.DryHandled{}, err
		}
		turn.confirm = false
		if ended := c.endedBy(res); ended != nil {
			return *ended, nil
		}
	}

	return engine.DryHandled{Outcome: workflowspec.OutcomeQuiet, Reason: "turn_limit"}, nil
}

/*
handedOver is an agent step that ended with a handover, and why.
*/
func handedOver(reason string) engine.DryHandled {
	return engine.DryHandled{Outcome: workflowspec.OutcomeHandedOver, Reason: reason}
}

/*
deref is how an agent step ended, nothing for nil.
*/
func deref(h *engine.DryHandled) engine.DryHandled {
	if h == nil {
		return engine.DryHandled{}
	}
	return *h
}

/*
answerAgent asks the customer for their answer to dimmi's reply. It
returns how the step ended when the customer is done or says nothing
more, and whether they pressed Confirm on a waiting card.
*/
func (c *chat) answerAgent(ctx context.Context, stepID string) (*engine.DryHandled, bool, error) {
	move, err := c.move(ctx, pipeline.CustomerTurn{CanWait: true, Confirming: c.pendingSummary()})
	if err != nil {
		return nil, false, err
	}
	switch {
	case move.Confirm:
		c.add(ctx, Line{From: "customer", Text: "(pressed Confirm)"})
		return nil, true, nil
	case move.Done:
		return &engine.DryHandled{Outcome: workflowspec.OutcomeResolved, Reason: "customer_done"}, false, nil
	case move.Waited || move.Message == "":
		c.add(ctx, Line{From: "customer", Silent: true, StepID: stepID})
		return &engine.DryHandled{Outcome: workflowspec.OutcomeQuiet, Reason: "customer_quiet"}, false, nil
	}
	c.customerSaid(ctx, move.Message, Line{})
	return nil, false, nil
}

/*
endedBy is how dimmi's turn ended the step: a handover, or a turn that
failed, which hands over as a real one does; nil when the chat goes on.
*/
func (c *chat) endedBy(res *pipeline.SimulationResult) *engine.DryHandled {
	if res.Handoff != nil {
		out := handedOver(res.Handoff.Reason)
		return &out
	}
	if res.Outcome == "error" || res.Outcome == "handoff" {
		c.handedOff = true
		out := handedOver("agent_" + res.Outcome)
		return &out
	}
	return nil
}

/*
agentAsk is one of dimmi's turns to play: the step, the agent it names,
and whether the customer's move was a press of Confirm.
*/
type agentAsk struct {
	stepID  string
	agent   *ids.UUID
	confirm bool
}

/*
pendingSummary is the card dimmi waits on the customer to confirm, if
any.
*/
func (c *chat) pendingSummary() string {
	if c.state == nil || len(c.state.Pending) == 0 {
		return ""
	}
	return c.state.Pending[0].Summary
}

/*
agentTurn answers the customer's waiting message as dimmi, following a
transfer to the agent it hands the chat to, and keeps the turn.
*/
func (c *chat) agentTurn(ctx context.Context, ask agentAsk) (*pipeline.SimulationResult, error) {
	m, stepID, agent, confirm := c.p.m, ask.stepID, ask.agent, ask.confirm
	at := len(c.turns)
	msg := c.waiting
	var confirmed []string
	if confirm && c.state != nil {
		for _, p := range c.state.Pending {
			confirmed = append(confirmed, p.ActionID)
		}
	}
	c.tell(ctx, "turn_started", map[string]any{"turn": at, "message": msg, "confirm": confirm, "step_id": stepID})
	answering := agent
	if c.heldBy != nil {
		answering = c.heldBy
	}
	history := withoutLast(c.history, msg)
	in := pipeline.SimulationInput{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, AgentID: answering, History: history, Message: msg,
		Persona: c.script.persona, Mocks: c.script.mocks, State: c.state, Confirmed: confirmed,
		StreamID: c.turnStream(at), Now: c.p.c.SimulatedAt,
	}
	res, err := c.s.Simulator.Simulate(ctx, in)
	var moved *pipeline.SimTransfer
	for hops := 0; err == nil && res.Transfer != nil && hops < maxTransfers; hops++ {
		moved = res.Transfer
		to := moved.To
		in.AgentID = &to
		res, err = c.s.Simulator.Simulate(ctx, in)
	}
	if err != nil {
		return nil, err
	}
	if res.Transfer == nil {
		res.Transfer = moved
	}
	if res.Transfer != nil {
		to := res.Transfer.To
		c.heldBy = &to
	}
	c.keepTurn(ctx, ask, msg, res)
	return res, nil
}

/*
keepTurn keeps one of dimmi's turns: the reply in the chat, the calls
and the handover in what was observed, and the turn itself.
*/
func (c *chat) keepTurn(ctx context.Context, ask agentAsk, msg string, res *pipeline.SimulationResult) {
	stepID, confirm := ask.stepID, ask.confirm
	if res.Model != "" && !slices.Contains(c.wrote, res.Model) {
		c.wrote = append(c.wrote, res.Model)
	}
	if res.Reply != "" {
		c.add(ctx, Line{From: "agent", Text: res.Reply, StepID: stepID, Name: res.AgentName})
		c.history = append(c.history, pipeline.SimMessage{From: "agent", Text: res.Reply})
	}
	c.calls = append(c.calls, res.Calls...)
	if h := res.Handoff; h != nil {
		c.handedOff, c.team, c.priority = true, h.TeamID, h.Priority
	}
	c.state, c.waiting = &res.State, ""
	turn := procedures.TestTurn{
		Message: msg, Confirm: confirm, Reply: res.Reply, Outcome: res.Outcome, Calls: res.Calls,
		ProcedureTrace: res.ProcedureTrace, RunID: models.PublicID(models.PrefixAiRun, res.RunID),
		Transfer: res.Transfer,
	}
	c.turns = append(c.turns, turn)
	c.tell(ctx, "turn_finished", map[string]any{"turn": len(c.turns) - 1, "result": turn, "step_id": stepID})
}

/*
withoutLast is the history without the customer's last message with
the text, which a turn sends as its message.
*/
func withoutLast(history []pipeline.SimMessage, text string) []pipeline.SimMessage {
	if text == "" {
		return history
	}
	for i, m := range slices.Backward(history) {
		if m.From == "customer" && m.Text == text {
			return append(append([]pipeline.SimMessage{}, history[:i]...), history[i+1:]...)
		}
	}
	return history
}

/*
stepAgent is the agent an agent step names, or the one placed where the
simulation's source conversation came in.
*/
func (c *chat) stepAgent(ctx context.Context, named *ids.UUID) *ids.UUID {
	if named != nil {
		return named
	}

	return c.placedAgent(ctx)
}

/*
placedAgent is the agent placed where the simulation's source
conversation came in, as a step that names none would get in a real
conversation; nil leaves it to the workspace's default.
*/
func (c *chat) placedAgent(ctx context.Context) *ids.UUID {
	source := c.p.c.SourceConversationID
	if source == nil || c.s.DB == nil {
		return nil
	}

	var placed *ids.UUID
	err := c.s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		conv, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: c.p.m.Workspace.ID, ID: *source,
		})
		if err != nil {
			return err
		}
		conv.AiAgentID = nil
		agent, err := agents.Resolve(ctx, q, agents.ForConversation(conv, nil))
		if err != nil {
			return err
		}
		placed = &agent.ID
		return nil
	})
	if err != nil {
		return nil
	}

	return placed
}
