/*
|--------------------------------------------------------------------------
| Procedure Test Cases
|--------------------------------------------------------------------------
|
| A saved test replays a made-up conversation against the working copy
| and checks what must hold after: the procedure matched, an action was
| or was not called, the order they ran in, the branch taken, a
| handover, words in the reply. Running the tests before publishing
| shows at once what a change broke. A test made from a real
| conversation, by a teammate or by Operator, names it.
|
*/

package procedures

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
Test case limits: the scripted messages, the tests one run takes, and
the saved tests of one procedure.
*/
const (
	maxTestMessages  = 12
	maxTestsPerRun   = 20
	maxTestCases     = 100
	maxScenarioRunes = 1000
)

/*
The failures of test cases.
*/
var (
	ErrTestCaseNotFound = fault.New(fault.NotFound, "procedure_test_case_not_found",
		"This procedure has no test with this id.")
	ErrTestCaseInvalid = fault.New(fault.Invalid, "procedure_test_case_invalid", "The test cannot be saved as it is.")
)

/*
TestMessage is one scripted customer message; Confirm first confirms
the writes waiting from the turn before, as the customer pressing the
card would.
*/
type TestMessage struct {
	Text    string `json:"text"`
	Confirm bool   `json:"confirm,omitempty"`
}

/*
TestCaseInput is a test to save. Nil fields of an update stay as they
are.
*/
type TestCaseInput struct {
	Name               *string
	SourceConversation *ids.UUID
	Persona            *pipeline.SimPersona
	Messages           []TestMessage
	Mocks              map[string]actionkit.Mock
	Assertions         []procedure.Assertion
	SimulatedAt        *time.Time
	Scenario           *string
}

/*
TestTurn is one turn of a test run. Transfer is set when the turn moved
the chat from one AI agent to another, which then wrote the reply.
*/
type TestTurn struct {
	Message        string                   `json:"message"`
	Confirm        bool                     `json:"confirm,omitempty"`
	Improvised     bool                     `json:"improvised,omitempty"`
	Reply          string                   `json:"reply"`
	Outcome        string                   `json:"outcome"`
	Calls          []procedure.ObservedCall `json:"calls"`
	ProcedureTrace json.RawMessage          `json:"procedure_trace,omitempty"`
	RunID          string                   `json:"run_id"`
	Transfer       *pipeline.SimTransfer    `json:"transfer,omitempty"`
}

/*
TestResult is how a test's run went.
*/
type TestResult struct {
	Passed           bool                        `json:"passed"`
	RanAt            time.Time                   `json:"ran_at"`
	Turns            []TestTurn                  `json:"turns"`
	AssertionResults []procedure.AssertionResult `json:"assertion_results"`
	Error            string                      `json:"error,omitempty"`
}

/*
TestCases lists a procedure's saved tests.
*/
func (p *Procedures) TestCases(ctx context.Context, m authz.Membership, id ids.UUID,
) ([]models.ProcedureTestCase, error) {
	if !m.Can("ai_actions:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.ProcedureTestCase
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		out, err = q.ListProcedureTestCases(ctx, models.ListProcedureTestCasesParams{
			WorkspaceID: m.Workspace.ID, ProcedureID: id,
		})

		return wrap("list procedure tests", err)
	})

	return out, err
}

/*
ShowTestCase returns one saved test.
*/
func (p *Procedures) ShowTestCase(ctx context.Context, m authz.Membership, id, testID ids.UUID,
) (models.ProcedureTestCase, error) {
	if !m.Can("ai_actions:read") {
		return models.ProcedureTestCase{}, exceptions.ErrForbidden
	}

	var out models.ProcedureTestCase
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findTestCase(ctx, models.New(tx), m.Workspace.ID, TestCaseRef{Procedure: id, Test: testID})

		return err
	})

	return out, err
}

/*
CreateTestCase saves a test of a procedure.
*/
func (p *Procedures) CreateTestCase(ctx context.Context, m authz.Membership, id ids.UUID, in TestCaseInput,
) (models.ProcedureTestCase, error) {
	if !m.Can("ai_agent:manage") {
		return models.ProcedureTestCase{}, exceptions.ErrForbidden
	}

	if in.Name == nil {
		return models.ProcedureTestCase{}, ErrTestCaseInvalid.With("reason", "Name the test.").OnField("name")
	}
	in = p.WithScenario(ctx, m, in)

	var out models.ProcedureTestCase
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		row, err := testRow(models.ProcedureTestCase{}, in)
		if err != nil {
			return err
		}

		if err := roomFor(ctx, q, procedureRef{ws: m.Workspace.ID, id: id}, in.SourceConversation); err != nil {
			return err
		}

		out, err = q.CreateProcedureTestCase(ctx, models.CreateProcedureTestCaseParams{
			WorkspaceID: m.Workspace.ID, ProcedureID: id, Name: row.Name, SourceConversationID: in.SourceConversation,
			Persona: row.Persona, Messages: row.Messages, ActionMocks: row.ActionMocks, Assertions: row.Assertions,
			SimulatedAt: row.SimulatedAt, Scenario: row.Scenario, MemberID: memberOf(m), Now: p.Clock.Now(),
		})

		return wrap("save procedure test", err)
	})

	return out, err
}

/*
roomFor refuses a new test past a procedure's limit, or one made from a
conversation that does not exist.
*/
func roomFor(ctx context.Context, q *models.Queries, ref procedureRef, source *ids.UUID) error {
	existing, err := q.ListProcedureTestCases(ctx, models.ListProcedureTestCasesParams{
		WorkspaceID: ref.ws, ProcedureID: ref.id,
	})
	if err != nil {
		return wrap("count procedure tests", err)
	}

	if len(existing) >= maxTestCases {
		return ErrTestCaseInvalid.With("reason", "A procedure keeps at most 100 tests.")
	}

	if source == nil {
		return nil
	}

	_, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ref.ws, ID: *source})
	if err != nil {
		return ErrTestCaseInvalid.With("reason", "The conversation does not exist.").OnField("source_conversation_id")
	}

	return nil
}

/*
TestCaseRef names one saved test of a procedure.
*/
type TestCaseRef struct {
	Procedure ids.UUID
	Test      ids.UUID
}

/*
UpdateTestCase saves a changed test.
*/
func (p *Procedures) UpdateTestCase(ctx context.Context, m authz.Membership, ref TestCaseRef, in TestCaseInput,
) (models.ProcedureTestCase, error) {
	id, testID := ref.Procedure, ref.Test
	if !m.Can("ai_agent:manage") {
		return models.ProcedureTestCase{}, exceptions.ErrForbidden
	}

	var out models.ProcedureTestCase
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findTestCase(ctx, q, m.Workspace.ID, TestCaseRef{Procedure: id, Test: testID})
		if err != nil {
			return err
		}

		row, err := testRow(current, in)
		if err != nil {
			return err
		}

		out, err = q.UpdateProcedureTestCase(ctx, models.UpdateProcedureTestCaseParams{
			Name: row.Name, Persona: row.Persona, Messages: row.Messages, ActionMocks: row.ActionMocks,
			Assertions: row.Assertions, SimulatedAt: row.SimulatedAt, Scenario: row.Scenario, Now: p.Clock.Now(),
			WorkspaceID: m.Workspace.ID, ProcedureID: id, ID: testID,
		})

		return wrap("save procedure test", err)
	})

	return out, err
}

/*
DeleteTestCase removes a saved test.
*/
func (p *Procedures) DeleteTestCase(ctx context.Context, m authz.Membership, id, testID ids.UUID) error {
	if !m.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}

	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findTestCase(ctx, q, m.Workspace.ID, TestCaseRef{Procedure: id, Test: testID}); err != nil {
			return err
		}

		return wrap("delete procedure test", q.DeleteProcedureTestCase(ctx, models.DeleteProcedureTestCaseParams{
			WorkspaceID: m.Workspace.ID, ProcedureID: id, ID: testID,
		}))
	})
}

/*
RunTestCases runs the named tests, or every one when none is named,
against the working copy, keeping each one's result.
*/
func (p *Procedures) RunTestCases(ctx context.Context, m authz.Membership, id ids.UUID, run TestRun,
) ([]models.ProcedureTestCase, error) {
	if !m.Can("ai_agent:manage") {
		return nil, exceptions.ErrForbidden
	}

	if p.Simulator == nil {
		return nil, ErrSimulatorUnavailable
	}

	cases, err := p.TestCases(ctx, m, id)
	if err != nil {
		return nil, err
	}

	if len(run.Only) > 0 {
		cases = slices.DeleteFunc(cases, func(c models.ProcedureTestCase) bool {
			return !slices.Contains(run.Only, c.ID)
		})
	}

	if len(cases) > maxTestsPerRun {
		return nil, ErrTestCaseInvalid.With("reason", "Run at most 20 tests at once.")
	}

	broken, err := p.workingCopyProblem(ctx, m, id)
	if err != nil {
		return nil, err
	}

	out := make([]models.ProcedureTestCase, 0, len(cases))
	for _, c := range cases {
		one := simulated{m: m, id: id, c: c, w: p.watching(m, run, c)}
		res := TestResult{RanAt: p.Clock.Now(), Turns: []TestTurn{}, Error: broken}
		if broken == "" {
			res = p.runTestCase(ctx, one)
		}
		kept, err := p.keepResult(ctx, one, res)
		if err != nil {
			return nil, err
		}
		out = append(out, kept)
	}

	return out, nil
}

/*
keepResult saves how a test's run went, tells whoever watches, and
returns the test with its new result.
*/
func (p *Procedures) keepResult(ctx context.Context, one simulated, res TestResult) (models.ProcedureTestCase, error) {
	m, w, c := one.m, one.w, one.c
	raw, _ := json.Marshal(res)
	w.tell(ctx, runFinished, map[string]any{"result": res})
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SetProcedureTestCaseResult(ctx, models.SetProcedureTestCaseResultParams{
			LastResult: raw, WorkspaceID: m.Workspace.ID, ID: c.ID,
		})
	})
	if err != nil {
		return c, wrap("keep test result", err)
	}
	c.LastResult = raw

	return c, nil
}

/*
testRow is a test's row with the input's changes, checked.
*/
func testRow(current models.ProcedureTestCase, in TestCaseInput) (models.ProcedureTestCase, error) {
	row := current
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
			return row, ErrTestCaseInvalid.With("reason", "Names are 1 to 120 characters.").OnField("name")
		}
		row.Name = name
	}

	if err := checkTestInput(in); err != nil {
		return row, err
	}

	if in.Persona != nil {
		row.Persona, _ = json.Marshal(in.Persona)
	}

	if in.Messages != nil {
		row.Messages, _ = json.Marshal(in.Messages)
	}

	if in.Mocks != nil {
		row.ActionMocks, _ = json.Marshal(in.Mocks)
	}

	if in.Assertions != nil {
		row.Assertions, _ = json.Marshal(in.Assertions)
	}

	if in.SimulatedAt != nil {
		row.SimulatedAt = in.SimulatedAt
	}

	if in.Scenario != nil {
		row.Scenario = strings.TrimSpace(*in.Scenario)
	}

	return withDefaults(row), nil
}

/*
checkTestInput refuses a persona on a channel dimmi does not answer
on, a script of the wrong length, and assertions that make no sense.
*/
func checkTestInput(in TestCaseInput) error {
	if in.Persona != nil && in.Persona.Channel != "" && !slices.Contains(procedure.Channels(), in.Persona.Channel) {
		return ErrTestCaseInvalid.With("reason", "Pick a channel dimmi answers on.").OnField("persona")
	}

	if in.Scenario != nil && utf8.RuneCountInString(strings.TrimSpace(*in.Scenario)) > maxScenarioRunes {
		return ErrTestCaseInvalid.With("reason", "Write the scenario in at most 1000 characters.").OnField("scenario")
	}

	if in.Messages != nil && (len(in.Messages) == 0 || len(in.Messages) > maxTestMessages) {
		return ErrTestCaseInvalid.With("reason", "Script 1 to 12 messages.").OnField("messages")
	}

	for _, msg := range in.Messages {
		n := utf8.RuneCountInString(strings.TrimSpace(msg.Text))
		if msg.Confirm && n == 0 {
			continue
		}
		if n == 0 || n > maxSimMessage {
			return ErrTestCaseInvalid.With("reason", "Write each message in 1 to 4,000 characters.").OnField("messages")
		}
	}

	if issues := procedure.CheckAssertions(in.Assertions); len(issues) > 0 {
		return ErrTestCaseInvalid.With("errors", issues).OnField("assertions")
	}

	return nil
}

/*
withDefaults fills a new test's empty parts.
*/
func withDefaults(row models.ProcedureTestCase) models.ProcedureTestCase {
	if len(row.Persona) == 0 {
		row.Persona = json.RawMessage("{}")
	}

	if len(row.Messages) == 0 {
		row.Messages = json.RawMessage("[]")
	}

	if len(row.ActionMocks) == 0 {
		row.ActionMocks = json.RawMessage("{}")
	}

	if len(row.Assertions) == 0 {
		row.Assertions = json.RawMessage("[]")
	}

	return row
}

/*
findTestCase returns one saved test of a procedure.
*/
func findTestCase(ctx context.Context, q *models.Queries, ws ids.UUID, ref TestCaseRef,
) (models.ProcedureTestCase, error) {
	if _, err := find(ctx, q, ws, ref.Procedure); err != nil {
		return models.ProcedureTestCase{}, err
	}

	out, err := q.FindProcedureTestCase(ctx, models.FindProcedureTestCaseParams{
		WorkspaceID: ws, ProcedureID: ref.Procedure, ID: ref.Test,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrTestCaseNotFound
	}

	return out, wrap("find procedure test", err)
}

/*
memberOf is the member acting, nil for a token without one.
*/
func memberOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}

	return &m.Member.ID
}
