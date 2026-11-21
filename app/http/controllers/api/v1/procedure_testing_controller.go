/*
|--------------------------------------------------------------------------
| Procedure Testing Controller
|--------------------------------------------------------------------------
|
| POST /procedures/{procedure}/simulate runs one simulated turn with
| mocked actions; .../test-cases manages saved tests and
| .../test-cases/run runs them against the working copy.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
ProcedureSimulationOutput is one simulated turn.
*/
type ProcedureSimulationOutput struct {
	Body fhttp.Resource[resources.ProcedureSimulation]
}

/*
ProcedureTestCasesOutput is a procedure's saved tests.
*/
type ProcedureTestCasesOutput struct {
	Body fhttp.Collection[resources.ProcedureTestCase]
}

/*
ProcedureTestCaseOutput is one saved test.
*/
type ProcedureTestCaseOutput struct {
	Body fhttp.Resource[resources.ProcedureTestCase]
}

/*
Simulate runs one simulated turn of a procedure.
*/
func (c *ProcedureController) Simulate(ctx context.Context, in *requests.SimulateProcedureRequest,
) (*ProcedureSimulationOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	b := in.Body
	sim := procedures.SimulateInput{
		Membership: membership(ctx), ID: id, Target: b.Target, Message: b.Message, Confirmed: b.Confirmed,
		Persona: persona(b.Persona), Mocks: mocks(b.ActionMocks), State: simState(b.ActiveState),
	}
	for _, h := range b.History {
		sim.History = append(sim.History, pipeline.SimMessage{From: h.Role, Text: h.Text})
	}

	if b.SourceConversationID != "" {
		conv, err := c.IDs.Decode(string(models.PrefixConversation), b.SourceConversationID)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}
		sim.SourceConversation = &conv
	}

	res, err := c.Procedures.Simulate(ctx, sim)
	if err != nil {
		return nil, err
	}

	return &ProcedureSimulationOutput{Body: fhttp.NewResource(resources.NewProcedureSimulation(res))}, nil
}

/*
TestCases lists a procedure's saved tests.
*/
func (c *ProcedureController) TestCases(ctx context.Context, in *requests.ProcedureRequest,
) (*ProcedureTestCasesOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	rows, err := c.Procedures.TestCases(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return testCasesOutput(rows), nil
}

/*
StoreTestCase saves a test of a procedure.
*/
func (c *ProcedureController) StoreTestCase(ctx context.Context, in *requests.StoreTestCaseRequest,
) (*ProcedureTestCaseOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	input, err := c.testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}

	row, err := c.Procedures.CreateTestCase(ctx, membership(ctx), id, input)

	return testCaseOutput(row, err)
}

/*
ShowTestCase answers one saved test.
*/
func (c *ProcedureController) ShowTestCase(ctx context.Context, in *requests.TestCaseRequest,
) (*ProcedureTestCaseOutput, error) {
	id, test, err := c.decodeTest(in.Procedure, in.TestCase)
	if err != nil {
		return nil, err
	}

	row, err := c.Procedures.ShowTestCase(ctx, membership(ctx), id, test)

	return testCaseOutput(row, err)
}

/*
UpdateTestCase saves a changed test.
*/
func (c *ProcedureController) UpdateTestCase(ctx context.Context, in *requests.UpdateTestCaseRequest,
) (*ProcedureTestCaseOutput, error) {
	id, test, err := c.decodeTest(in.Procedure, in.TestCase)
	if err != nil {
		return nil, err
	}

	input, err := c.testCaseInput(in.Body)
	if err != nil {
		return nil, err
	}

	row, err := c.Procedures.UpdateTestCase(ctx, membership(ctx), procedures.TestCaseRef{Procedure: id, Test: test},
		input)

	return testCaseOutput(row, err)
}

/*
DestroyTestCase removes a saved test.
*/
func (c *ProcedureController) DestroyTestCase(ctx context.Context, in *requests.TestCaseRequest) (*EmptyOutput, error) {
	id, test, err := c.decodeTest(in.Procedure, in.TestCase)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Procedures.DeleteTestCase(ctx, membership(ctx), id, test)
}

/*
RunTestCases runs saved tests against the working copy.
*/
func (c *ProcedureController) RunTestCases(ctx context.Context, in *requests.RunTestCasesRequest,
) (*ProcedureTestCasesOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	var only []ids.UUID
	if in.Body != nil {
		for _, public := range in.Body.TestCaseIDs {
			test, err := c.IDs.Decode(string(models.PrefixProcedureTest), public)
			if err != nil {
				return nil, procedures.ErrTestCaseNotFound
			}
			only = append(only, test)
		}
	}

	rows, err := c.Procedures.RunTestCases(ctx, membership(ctx), id, only)
	if err != nil {
		return nil, err
	}

	return testCasesOutput(rows), nil
}

/*
testCaseInput reads a saved test's fields.
*/
func (c *ProcedureController) testCaseInput(b requests.TestCaseBody) (procedures.TestCaseInput, error) {
	out := procedures.TestCaseInput{Name: b.Name, Mocks: mocks(b.ActionMocks)}
	if b.Persona != nil {
		p := persona(b.Persona)
		out.Persona = &p
	}

	if b.Messages != nil {
		out.Messages = make([]procedures.TestMessage, len(b.Messages))
		for i, m := range b.Messages {
			out.Messages[i] = procedures.TestMessage{Text: m.Text, Confirm: m.Confirm}
		}
	}

	if b.Assertions != nil {
		raw, _ := json.Marshal(b.Assertions)
		var as []procedure.Assertion
		if err := json.Unmarshal(raw, &as); err != nil {
			return out, procedures.ErrTestCaseInvalid.With("reason", err.Error()).OnField("assertions")
		}
		out.Assertions = as
	}

	if b.SourceConversationID != "" {
		conv, err := c.IDs.Decode(string(models.PrefixConversation), b.SourceConversationID)
		if err != nil {
			return out, procedures.ErrTestCaseInvalid.With("reason", "The conversation does not exist.").
				OnField("source_conversation_id")
		}
		out.SourceConversation = &conv
	}

	return out, nil
}

/*
decodeTest reads a procedure's and its test's public ids.
*/
func (c *ProcedureController) decodeTest(public, test string) (ids.UUID, ids.UUID, error) {
	id, err := c.decode(public)
	if err != nil {
		return id, ids.UUID{}, err
	}

	testID, err := c.IDs.Decode(string(models.PrefixProcedureTest), test)
	if err != nil {
		return id, testID, procedures.ErrTestCaseNotFound
	}

	return id, testID, nil
}

/*
persona reads a simulated customer.
*/
func persona(in *requests.SimPersonaInput) pipeline.SimPersona {
	if in == nil {
		return pipeline.SimPersona{}
	}

	return pipeline.SimPersona{
		Name: in.Name, Email: in.Email, ExternalID: in.ExternalID, Verified: in.Verified, Channel: in.Channel,
		InboxID: in.InboxID, Attributes: in.Attributes, Company: in.Company,
	}
}

/*
mocks reads the mocked actions by public id.
*/
func mocks(in map[string]requests.ActionMockInput) map[string]actionkit.Mock {
	if in == nil {
		return nil
	}

	out := make(map[string]actionkit.Mock, len(in))
	for id, m := range in {
		out[id] = actionkit.Mock{Status: m.Status, Body: m.Body}
	}

	return out
}

/*
simState reads the state the last simulated turn handed back.
*/
func simState(in *requests.SimStateInput) *pipeline.SimState {
	if in == nil {
		return nil
	}

	out := &pipeline.SimState{
		ProcedureID: in.ProcedureID, Status: in.Status, Called: in.CalledActionIDs, Outputs: in.StepOutputs,
	}
	for _, p := range in.Pending {
		out.Pending = append(out.Pending, pipeline.SimPending{
			ActionID: p.ActionID, Tool: p.Tool, Args: p.Args, State: p.State, Summary: p.Summary,
		})
	}

	return out
}

func testCasesOutput(rows []models.ProcedureTestCase) *ProcedureTestCasesOutput {
	page := pagination.Page[resources.ProcedureTestCase]{Items: resources.NewProcedureTestCases(rows), Limit: 100}

	return &ProcedureTestCasesOutput{Body: fhttp.NewCollection(page)}
}

func testCaseOutput(row models.ProcedureTestCase, err error) (*ProcedureTestCaseOutput, error) {
	if err != nil {
		return nil, err
	}

	return &ProcedureTestCaseOutput{Body: fhttp.NewResource(resources.NewProcedureTestCase(row))}, nil
}
