/*
|--------------------------------------------------------------------------
| Workflow Simulation Controller
|--------------------------------------------------------------------------
|
| /workflows/{workflow}/simulations keeps the chats a workflow must keep
| handling the way the team means: list, save, change and delete them,
| save a real conversation as one, run them against the draft or the
| live version, and read a simulation's past runs. POST
| /series/{series}/simulations/run walks one person through a series
| without sending anything.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/actions/workflowsims"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowSimulationController serves workflows' saved simulations and
series' dry simulations.
*/
type WorkflowSimulationController struct {
	Simulations *workflowsims.Simulations
	Series      *series.Series
	IDs         *ids.Registry
}

/*
WorkflowSimulationsOutput is a workflow's saved simulations.
*/
type WorkflowSimulationsOutput struct {
	Body fhttp.Collection[resources.WorkflowSimulation]
}

/*
WorkflowSimulationOutput is one saved simulation.
*/
type WorkflowSimulationOutput struct {
	Body fhttp.Resource[resources.WorkflowSimulation]
}

/*
WorkflowSimulationRunsOutput is a simulation's kept runs.
*/
type WorkflowSimulationRunsOutput struct {
	Body fhttp.Collection[resources.WorkflowSimulationRun]
}

/*
SeriesSimulationOutput is a series' dry simulation.
*/
type SeriesSimulationOutput struct {
	Body fhttp.Resource[resources.SeriesSimulation]
}

/*
Index lists a workflow's saved simulations.
*/
func (c *WorkflowSimulationController) Index(ctx context.Context, in *requests.WorkflowRequest,
) (*WorkflowSimulationsOutput, error) {
	wf, err := c.workflowID(in.Workflow)
	if err != nil {
		return nil, err
	}
	rows, err := c.Simulations.List(ctx, membership(ctx), wf)
	if err != nil {
		return nil, err
	}
	return workflowSimulationsOutput(rows), nil
}

/*
Store saves a simulation of a workflow.
*/
func (c *WorkflowSimulationController) Store(ctx context.Context, in *requests.StoreWorkflowSimulationRequest,
) (*WorkflowSimulationOutput, error) {
	wf, err := c.workflowID(in.Workflow)
	if err != nil {
		return nil, err
	}
	input, err := (&ProcedureController{IDs: c.IDs}).testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}
	return workflowSimulationOutput(c.Simulations.Create(ctx, membership(ctx), wf, input))
}

/*
Import saves a real conversation as a simulation.
*/
func (c *WorkflowSimulationController) Import(ctx context.Context, in *requests.ImportWorkflowSimulationRequest,
) (*WorkflowSimulationOutput, error) {
	wf, err := c.workflowID(in.Workflow)
	if err != nil {
		return nil, err
	}
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Body.ConversationID)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	return workflowSimulationOutput(c.Simulations.FromConversation(ctx, membership(ctx), wf, conv))
}

/*
Show answers one saved simulation.
*/
func (c *WorkflowSimulationController) Show(ctx context.Context, in *requests.WorkflowSimulationRequest,
) (*WorkflowSimulationOutput, error) {
	ref, err := c.ref(in.Workflow, in.Simulation)
	if err != nil {
		return nil, err
	}
	return workflowSimulationOutput(c.Simulations.Show(ctx, membership(ctx), ref))
}

/*
Update saves a changed simulation.
*/
func (c *WorkflowSimulationController) Update(ctx context.Context, in *requests.UpdateWorkflowSimulationRequest,
) (*WorkflowSimulationOutput, error) {
	ref, err := c.ref(in.Workflow, in.Simulation)
	if err != nil {
		return nil, err
	}
	input, err := (&ProcedureController{IDs: c.IDs}).testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}
	return workflowSimulationOutput(c.Simulations.Update(ctx, membership(ctx), ref, input))
}

/*
Destroy deletes a saved simulation.
*/
func (c *WorkflowSimulationController) Destroy(ctx context.Context, in *requests.WorkflowSimulationRequest,
) (*EmptyOutput, error) {
	ref, err := c.ref(in.Workflow, in.Simulation)
	if err != nil {
		return nil, err
	}
	return &EmptyOutput{}, c.Simulations.Delete(ctx, membership(ctx), ref)
}

/*
Run plays a workflow's simulations against its draft or live version.
*/
func (c *WorkflowSimulationController) Run(ctx context.Context, in *requests.RunWorkflowSimulationsRequest,
) (*WorkflowSimulationsOutput, error) {
	wf, err := c.workflowID(in.Workflow)
	if err != nil {
		return nil, err
	}
	var run workflowsims.RunInput
	if b := in.Body; b != nil {
		run.Version, run.StreamID = b.Version, b.StreamID
		for _, public := range b.SimulationIDs {
			id, err := c.IDs.Decode(string(models.PrefixWorkflowSim), public)
			if err != nil {
				return nil, workflowsims.ErrSimulationNotFound
			}
			run.Only = append(run.Only, id)
		}
	}
	rows, err := c.Simulations.Run(ctx, membership(ctx), wf, run)
	if err != nil {
		return nil, err
	}
	return workflowSimulationsOutput(rows), nil
}

/*
Runs lists a simulation's latest runs on the draft or the live version,
newest first.
*/
func (c *WorkflowSimulationController) Runs(ctx context.Context, in *requests.WorkflowSimulationRunsRequest,
) (*WorkflowSimulationRunsOutput, error) {
	ref, err := c.ref(in.Workflow, in.Simulation)
	if err != nil {
		return nil, err
	}
	rows, err := c.Simulations.Runs(ctx, membership(ctx), ref, in.Version)
	if err != nil {
		return nil, err
	}
	items := resources.NewWorkflowSimulationRuns(rows)
	page := pagination.Page[resources.WorkflowSimulationRun]{Items: items, Limit: len(items)}
	return &WorkflowSimulationRunsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
SimulateSeries walks one person through a series without sending
anything.
*/
func (c *WorkflowSimulationController) SimulateSeries(ctx context.Context, in *requests.RunSeriesSimulationRequest,
) (*SeriesSimulationOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixSeries), in.Series)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	b := in.Body
	input := series.SimulateInput{
		Membership: membership(ctx), ID: id, Version: b.Version, Events: b.Events, Engaged: b.Engaged,
		Mocks: mocks(b.ActionMocks),
	}
	if b.ContactID != "" {
		contact, err := c.IDs.Decode(string(models.PrefixContact), b.ContactID)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}
		input.ContactID = &contact
	}
	if p := b.Person; p != nil {
		input.Person = &series.SimPerson{Name: p.Name, Email: p.Email, Attributes: p.Attributes}
	}
	trace, err := c.Series.Simulate(ctx, input)
	if err != nil {
		return nil, err
	}
	return &SeriesSimulationOutput{Body: fhttp.NewResource(resources.NewSeriesSimulation(trace))}, nil
}

/*
workflowID decodes a workflow's public id; anything else is not found.
*/
func (c *WorkflowSimulationController) workflowID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixWorkflow), public)
	if err != nil {
		return ids.UUID{}, workflowsims.ErrWorkflowNotFound
	}
	return id, nil
}

/*
ref decodes a workflow's and its simulation's public ids.
*/
func (c *WorkflowSimulationController) ref(workflow, simulation string) (workflowsims.SimulationRef, error) {
	wf, err := c.workflowID(workflow)
	if err != nil {
		return workflowsims.SimulationRef{}, err
	}
	s, err := c.IDs.Decode(string(models.PrefixWorkflowSim), simulation)
	if err != nil {
		return workflowsims.SimulationRef{}, workflowsims.ErrSimulationNotFound
	}
	return workflowsims.SimulationRef{Workflow: wf, Simulation: s}, nil
}

/*
workflowSimulationOutput shapes one saved simulation, or passes the
error on.
*/
func workflowSimulationOutput(row models.WorkflowSimulation, err error) (*WorkflowSimulationOutput, error) {
	if err != nil {
		return nil, err
	}
	return &WorkflowSimulationOutput{Body: fhttp.NewResource(resources.NewWorkflowSimulation(row))}, nil
}

/*
workflowSimulationsOutput shapes a list of saved simulations.
*/
func workflowSimulationsOutput(rows []models.WorkflowSimulation) *WorkflowSimulationsOutput {
	items := resources.NewWorkflowSimulations(rows)
	page := pagination.Page[resources.WorkflowSimulation]{Items: items, Limit: len(items)}
	return &WorkflowSimulationsOutput{Body: fhttp.NewCollection(page)}
}
