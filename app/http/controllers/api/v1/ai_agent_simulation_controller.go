/*
|--------------------------------------------------------------------------
| AI Agent Simulation Controller
|--------------------------------------------------------------------------
|
| POST /ai/agents/{agent}/preview answers one turn as the agent's draft
| or live version. .../simulations keeps the chats it must keep getting
| right: list, save, change and delete them, save a real conversation as
| one, and run them against either copy.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentsims"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAgentSimulationController serves agents' previews and simulations.
*/
type AIAgentSimulationController struct {
	Simulations *agentsims.Simulations
	IDs         *ids.Registry
}

/*
AIAgentPreviewOutput is one preview turn.
*/
type AIAgentPreviewOutput struct {
	Body fhttp.Resource[resources.AIAgentPreview]
}

/*
AIAgentSimulationsOutput is an agent's saved simulations.
*/
type AIAgentSimulationsOutput struct {
	Body fhttp.Collection[resources.AIAgentSimulation]
}

/*
AIAgentSimulationOutput is one saved simulation.
*/
type AIAgentSimulationOutput struct {
	Body fhttp.Resource[resources.AIAgentSimulation]
}

/*
Preview answers one turn as the agent.
*/
func (c *AIAgentSimulationController) Preview(ctx context.Context, in *requests.AIPreviewAgentRequest,
) (*AIAgentPreviewOutput, error) {
	agent, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	b := in.Body
	input := agentsims.PreviewInput{
		Membership: membership(ctx), AgentID: agent, Version: b.Version, Message: b.Message,
		Persona: persona(b.Persona), Mocks: mocks(b.ActionMocks), State: simState(b.ActiveState),
		Confirmed: b.Confirmed, StreamID: b.StreamID,
	}
	for _, h := range b.History {
		input.History = append(input.History, pipeline.SimMessage{From: h.Role, Text: h.Text})
	}
	if b.SourceConversationID != "" {
		conv, err := c.IDs.Decode(string(models.PrefixConversation), b.SourceConversationID)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}
		input.SourceConversation = &conv
	}
	res, err := c.Simulations.Preview(ctx, input)
	if err != nil {
		return nil, err
	}
	return &AIAgentPreviewOutput{Body: fhttp.NewResource(resources.NewAIAgentPreview(res))}, nil
}

/*
Index lists an agent's saved simulations.
*/
func (c *AIAgentSimulationController) Index(ctx context.Context, in *requests.AIShowAgentRequest,
) (*AIAgentSimulationsOutput, error) {
	agent, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	rows, err := c.Simulations.List(ctx, membership(ctx), agent)
	if err != nil {
		return nil, err
	}
	return simulationsOutput(rows), nil
}

/*
Store saves a simulation of an agent.
*/
func (c *AIAgentSimulationController) Store(ctx context.Context, in *requests.AIStoreAgentSimulationRequest,
) (*AIAgentSimulationOutput, error) {
	agent, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	input, err := (&ProcedureController{IDs: c.IDs}).testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}
	return simulationOutput(c.Simulations.Create(ctx, membership(ctx), agent, input))
}

/*
Import saves a real conversation as a simulation.
*/
func (c *AIAgentSimulationController) Import(ctx context.Context, in *requests.AIImportAgentSimulationRequest,
) (*AIAgentSimulationOutput, error) {
	agent, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Body.ConversationID)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	return simulationOutput(c.Simulations.FromConversation(ctx, membership(ctx), agent, conv))
}

/*
Show answers one saved simulation.
*/
func (c *AIAgentSimulationController) Show(ctx context.Context, in *requests.AIAgentSimulationRequest,
) (*AIAgentSimulationOutput, error) {
	ref, err := c.ref(in.Agent, in.Simulation)
	if err != nil {
		return nil, err
	}
	return simulationOutput(c.Simulations.Show(ctx, membership(ctx), ref))
}

/*
Update saves a changed simulation.
*/
func (c *AIAgentSimulationController) Update(ctx context.Context, in *requests.AIUpdateAgentSimulationRequest,
) (*AIAgentSimulationOutput, error) {
	ref, err := c.ref(in.Agent, in.Simulation)
	if err != nil {
		return nil, err
	}
	input, err := (&ProcedureController{IDs: c.IDs}).testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}
	return simulationOutput(c.Simulations.Update(ctx, membership(ctx), ref, input))
}

/*
Destroy deletes a saved simulation.
*/
func (c *AIAgentSimulationController) Destroy(ctx context.Context, in *requests.AIAgentSimulationRequest,
) (*EmptyOutput, error) {
	ref, err := c.ref(in.Agent, in.Simulation)
	if err != nil {
		return nil, err
	}
	return &EmptyOutput{}, c.Simulations.Delete(ctx, membership(ctx), ref)
}

/*
Run plays an agent's simulations against its draft or live version.
*/
func (c *AIAgentSimulationController) Run(ctx context.Context, in *requests.AIRunAgentSimulationsRequest,
) (*AIAgentSimulationsOutput, error) {
	agent, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	var run agentsims.RunInput
	if b := in.Body; b != nil {
		run.Version, run.StreamID = b.Version, b.StreamID
		for _, public := range b.SimulationIDs {
			id, err := c.IDs.Decode(string(models.PrefixAiAgentTestCase), public)
			if err != nil {
				return nil, agentsims.ErrSimulationNotFound
			}
			run.Only = append(run.Only, id)
		}
	}
	rows, err := c.Simulations.Run(ctx, membership(ctx), agent, run)
	if err != nil {
		return nil, err
	}
	return simulationsOutput(rows), nil
}

/*
agentID decodes an agent's public id; anything else is not found.
*/
func (c *AIAgentSimulationController) agentID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAiAgent), public)
	if err != nil {
		return ids.UUID{}, ai.ErrAgentNotFound
	}
	return id, nil
}

/*
ref decodes an agent's and its simulation's public ids.
*/
func (c *AIAgentSimulationController) ref(agent, simulation string) (agentsims.SimulationRef, error) {
	a, err := c.agentID(agent)
	if err != nil {
		return agentsims.SimulationRef{}, err
	}
	s, err := c.IDs.Decode(string(models.PrefixAiAgentTestCase), simulation)
	if err != nil {
		return agentsims.SimulationRef{}, agentsims.ErrSimulationNotFound
	}
	return agentsims.SimulationRef{Agent: a, Simulation: s}, nil
}

/*
simulationOutput shapes one saved simulation, or passes the error on.
*/
func simulationOutput(row models.AiAgentTestCase, err error) (*AIAgentSimulationOutput, error) {
	if err != nil {
		return nil, err
	}
	return &AIAgentSimulationOutput{Body: fhttp.NewResource(resources.NewAIAgentSimulation(row))}, nil
}

/*
simulationsOutput shapes a list of saved simulations.
*/
func simulationsOutput(rows []models.AiAgentTestCase) *AIAgentSimulationsOutput {
	items := resources.NewAIAgentSimulations(rows)
	page := pagination.Page[resources.AIAgentSimulation]{Items: items, Limit: len(items)}
	return &AIAgentSimulationsOutput{Body: fhttp.NewCollection(page)}
}
