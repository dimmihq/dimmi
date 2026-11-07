/*
|--------------------------------------------------------------------------
| Action Tests
|--------------------------------------------------------------------------
|
| An admin tries an action's working copy before publishing it, with
| sample input and a stand-in customer treated as verified. A dry run
| renders the request, credential masked, and sends nothing. A live run
| sends it and shows the request, the response and exactly what the
| model would read; a live run of a write must be confirmed explicitly,
| since it changes a real account. An upstream failure is a result to
| show, never an error of the endpoint.
|
*/

package agentactions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionschema"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
TestInput is one test: the action, the sample input, the stand-in
customer, dry_run or live, and the explicit yes a live write needs.
*/
type TestInput struct {
	Membership         authz.Membership
	ActionID           ids.UUID
	Input              json.RawMessage
	Contact            TestContact
	Mode               string
	ConfirmSideEffects bool
}

/*
TestContact is the stand-in customer a test binds to.
*/
type TestContact struct {
	ExternalID string         `json:"external_id,omitempty"`
	Email      string         `json:"email,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

/*
TestResult is how a test went: rendered, succeeded, failed or blocked,
the error code, the request and response with secrets masked, what the
model would read, the summary a customer would confirm, and the call
recorded for a live run.
*/
type TestResult struct {
	Status           string
	ErrorCode        string
	ValidationErrors []string
	Request          json.RawMessage
	Response         json.RawMessage
	Result           json.RawMessage
	Summary          string
	Latency          time.Duration
	CallID           *ids.UUID
}

/*
Test runs a test of the action's working copy.
*/
func (d *Definitions) Test(ctx context.Context, in TestInput) (TestResult, error) {
	if err := RequireManage(in.Membership); err != nil {
		return TestResult{}, err
	}
	if in.Mode != "live" && in.Mode != "dry_run" {
		return TestResult{}, invalidField("mode_invalid", "mode", "mode is dry_run or live")
	}
	target, err := d.testTarget(ctx, in)
	if err != nil {
		return TestResult{}, err
	}
	if target.result != nil {
		return *target.result, nil
	}
	t := target.target
	summary := actionkit.Summarize(t.Spec, t.Input)
	if in.Mode == "dry_run" {
		request, err := actionkit.DryRun(t)
		if err != nil {
			return TestResult{Status: "blocked", ErrorCode: actionkit.RenderCode(err), Summary: summary}, nil
		}
		return TestResult{Status: "rendered", Request: request, Summary: summary}, nil
	}
	if t.Spec.SideEffect == actionkit.SideEffectWrite && !in.ConfirmSideEffects {
		return TestResult{}, exceptions.ErrSideEffectsNotConfirmed.OnField("confirm_side_effects")
	}
	return d.runTest(ctx, target, summary)
}

/*
prepared is a test ready to run, with the action it tests, or the
result it already came to.
*/
type prepared struct {
	actionID ids.UUID
	target   actionkit.Target
	result   *TestResult
}

/*
testTarget loads the working copy and checks the sample input and
stand-in customer against it.
*/
func (d *Definitions) testTarget(ctx context.Context, in TestInput) (prepared, error) {
	var a models.AiAction
	var conn models.AiActionConnection
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		v, err := loadView(ctx, tx, in.Membership.Workspace.ID, in.ActionID)
		if err != nil {
			return err
		}
		a = v.Action
		conn, err = findConnection(ctx, tx, a.WorkspaceID, a.ConnectionID)
		return err
	})
	if err != nil {
		return prepared{}, err
	}
	s, err := actionkit.Normalize(actionkit.FromAction(a), conn)
	if err != nil {
		return prepared{}, err
	}
	input, refused := validateInput(s, in.Input)
	if refused != nil {
		return prepared{result: refused}, nil
	}
	contact := testContact(in.Contact)
	if s.UsesContact() && !bound(s, contact) {
		return prepared{result: &TestResult{Status: "blocked", ErrorCode: "identity_unavailable"}}, nil
	}
	creds, err := actionkit.OpenCredentials(ctx, d.Kit.Keys, conn)
	if err != nil {
		return prepared{}, err
	}
	return prepared{actionID: a.ID, target: actionkit.Target{
		Conn: conn, Creds: creds, Spec: s, Input: input, Bindings: actionkit.Bindings{Contact: contact},
		CallID: ids.New(), WorkspaceID: a.WorkspaceID,
	}}, nil
}

/*
runTest records the test call, sends it and records what came back.
*/
func (d *Definitions) runTest(ctx context.Context, p prepared, summary string) (TestResult, error) {
	t := p.target
	input, _ := json.Marshal(t.Input)
	var call models.AiActionCall
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := d.Clock.Now()
		var err error
		call, err = models.New(tx).CreateActionCall(ctx, models.CreateActionCallParams{
			WorkspaceID: t.WorkspaceID, ActionID: p.actionID, ConnectionID: t.Conn.ID, Mode: "test",
			SideEffect: t.Spec.SideEffect, Confirmation: actionkit.ConfirmationNone, Approval: actionkit.ApprovalNone,
			State: "running", Title: t.Spec.Title, Summary: summary, Input: input, Bindings: json.RawMessage("{}"),
			SentAt: &now, Attempts: 1, Now: now,
		})
		return err
	})
	if err != nil {
		return TestResult{}, fmt.Errorf("record test call: %w", err)
	}
	t.CallID = call.ID
	out := d.Kit.Send(ctx, t)
	if _, err := d.Kit.FinishTest(ctx, call, out); err != nil {
		return TestResult{}, err
	}
	status := "failed"
	if out.Succeeded {
		status = "succeeded"
	}
	return TestResult{
		Status: status, ErrorCode: out.ErrorCode, Request: out.Request, Response: out.Response, Result: out.Result,
		Summary: summary, Latency: out.Latency, CallID: &call.ID,
	}, nil
}

/*
validateInput checks the sample input against the schema, returning the
failed result when it does not fit.
*/
func validateInput(s actionkit.Spec, raw json.RawMessage) (map[string]any, *TestResult) {
	schema, err := actionschema.Compile(s.ModelSchema())
	if err == nil {
		var input map[string]any
		if input, err = schema.Validate(raw); err == nil {
			return input, nil
		}
	}
	return nil, &TestResult{Status: "failed", ErrorCode: "invalid_arguments", ValidationErrors: []string{err.Error()}}
}

/*
testContact turns the stand-in customer into contact values.
*/
func testContact(c TestContact) map[string]any {
	out := map[string]any{actiontemplate.ContactAttr: map[string]any{}}
	if c.ExternalID != "" {
		out[actiontemplate.ContactExternalID] = c.ExternalID
	}
	if c.Email != "" {
		out[actiontemplate.ContactEmail] = c.Email
	}
	if c.Attributes != nil {
		out[actiontemplate.ContactAttr] = c.Attributes
	}
	return out
}

/*
bound reports whether every contact value the action binds is there.
*/
func bound(s actionkit.Spec, contact map[string]any) bool {
	vars := actiontemplate.Vars{Contact: contact}
	for _, r := range s.Refs() {
		if _, ok := vars.Lookup(r); r.UsesContact() && !ok {
			return false
		}
	}
	return true
}
