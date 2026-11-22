/*
|--------------------------------------------------------------------------
| Simulated Agent Turns
|--------------------------------------------------------------------------
|
| One simulated turn answered as a copy of an agent. A turn that hands
| the chat to another agent goes on as that agent's live version, the
| way a real conversation does, so a simulation sees who answered.
|
*/

package agentsims

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

/*
turn answers simulated turns as one copy of an agent.
*/
func (s *Simulations) turn(agentID ids.UUID, version string) procedures.TurnFunc {
	return func(ctx context.Context, in procedures.SimulateInput) (*pipeline.SimulationResult, error) {
		if s.Simulator == nil {
			return nil, ErrSimulatorOff
		}
		def, err := s.copyOf(ctx, in.Membership, agentID, version)
		if err != nil {
			return nil, err
		}
		res, err := s.simulate(ctx, in, agentID, def)
		for hops := 0; err == nil && res.Transfer != nil && hops < maxSimTransfers; hops++ {
			moved := res.Transfer
			if res, err = s.simulate(ctx, in, moved.To, nil); err == nil && res.Transfer == nil {
				res.Transfer = moved
			}
		}
		return res, err
	}
}

/*
maxSimTransfers is how many transfers one simulated turn follows.
*/
const maxSimTransfers = 2

/*
simulate answers one simulated turn as an agent's copy: def, or its
live version when def is nil.
*/
func (s *Simulations) simulate(ctx context.Context, in procedures.SimulateInput, agentID ids.UUID,
	def *agents.Definition,
) (*pipeline.SimulationResult, error) {
	m := in.Membership
	res, err := s.Simulator.Simulate(ctx, pipeline.SimulationInput{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, AgentID: &agentID, Agent: def,
		History: in.History, Message: in.Message, Persona: in.Persona, Mocks: in.Mocks, State: in.State,
		Confirmed: in.Confirmed, StreamID: in.StreamID, Now: in.Now,
	})
	if errors.Is(err, pipeline.ErrNoAgent) {
		return nil, ErrSimulatorOff
	}
	return res, err
}
