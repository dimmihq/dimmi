/*
|--------------------------------------------------------------------------
| Mocked Calls
|--------------------------------------------------------------------------
|
| The procedure simulator runs the agent's whole turn, but no action it
| calls ever reaches the team's systems: not a byte leaves dimmi. The
| arguments are checked and the customer's identity bound exactly as a
| real call would, then the action answers with the mock the test gave,
| or the result its last live test returned, shaped as the model would
| read a real one. A write that needs the customer's confirmation or a
| teammate's approval waits, as it would, until the simulator says it
| was confirmed or approved.
|
*/

package actionkit

import (
	"encoding/json"
	"strconv"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Mock is what a mocked action answers: an HTTP status and a body, which
is shaped by the action's response fields unless Shaped says it already
is the result the model reads.
*/
type Mock struct {
	Status int             `json:"status,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
	Shaped bool            `json:"-"`
}

/*
MockInput is one mocked call: the offered action, the model's
arguments, who the customer is, the mock, and whether the simulator
already confirmed or approved it.
*/
type MockInput struct {
	Live     Live
	Args     json.RawMessage
	Identity Identity
	Mock     *Mock
	Agreed   bool
}

/*
MockReply is the reply of a mocked call, with the validated arguments
and the result the model read.
*/
type MockReply struct {
	CallReply
	Input  map[string]any
	Result json.RawMessage
}

/*
MockCall answers one call without running it.
*/
func MockCall(in MockInput) MockReply {
	s := in.Live.Spec
	input, refused := validate(s, in.Args)
	if refused != nil {
		return MockReply{CallReply: *refused}
	}
	contact := ContactValues(in.Identity)
	if s.UsesContact() && !bindable(s, contact) {
		return MockReply{
			CallReply: refusal(s, "identity_unavailable", "not available: the customer is not signed in"), Input: input,
		}
	}

	call := models.AiActionCall{
		ID: ids.New(), ActionID: in.Live.ActionID, Title: s.Title, Summary: Summarize(s, input),
		SideEffect: s.SideEffect, Confirmation: s.Confirmation, Approval: s.Approval, Mode: "simulation",
	}
	if s.NeedsAgreement() && !in.Agreed {
		call.State = "pending_approval"
		if s.Confirmation != ConfirmationNone {
			call.State = "awaiting_confirmation"
		}
		return MockReply{
			CallReply: CallReply{
				Text: pendingText(s, call), Call: &call, Awaiting: call.State == "awaiting_confirmation",
				Pending: call.State == "pending_approval",
			},
			Input: input,
		}
	}

	out := mockOutcome(s, in.Mock)
	if in.Mock != nil && in.Mock.Shaped && out.Succeeded {
		out.Result = echo(out.Result, input)
	}
	call.State = "failed"
	if out.Succeeded {
		call.State = "succeeded"
	}
	return MockReply{
		CallReply: CallReply{
			Text: wrap(s, call, out.Result), Call: &call, Succeeded: out.Succeeded, Failure: failure(out),
		},
		Input: input, Result: out.Result,
	}
}

/*
echo writes the arguments of a call over the fields of a recorded
result that carry the same names, so a result kept from an earlier test
answers for what was asked this time: changing to the Starter plan does
not come back as the plan the last test picked.
*/
func echo(result json.RawMessage, input map[string]any) json.RawMessage {
	var fields map[string]any
	if json.Unmarshal(result, &fields) != nil {
		return result
	}
	changed := false
	for name, value := range input {
		if _, ok := fields[name]; ok {
			fields[name], changed = value, true
		}
	}
	if !changed {
		return result
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return result
	}
	return out
}

/*
mockOutcome turns a mock into the outcome a real response would give;
no mock answers like an empty success.
*/
func mockOutcome(s Spec, m *Mock) Outcome {
	if m == nil {
		return Outcome{Succeeded: true, Result: json.RawMessage(`{"ok":true}`)}
	}
	status := m.Status
	if status == 0 {
		status = 200
	}
	if m.Shaped && status < 300 {
		return Outcome{Succeeded: true, Result: m.Body}
	}
	out := Outcome{HTTPStatus: status}
	switch {
	case status >= 200 && status < 300:
		out.Succeeded, out.Result = true, shape(m.Body, s)
	case status == 404:
		out.ErrorCode, out.Message = "not_found", upstreamMessage(m.Body, s, "not found")
	case status == 400 || status == 409 || status == 422:
		out.ErrorCode, out.Message = "upstream_rejected", upstreamMessage(m.Body, s, "the request was refused")
	case status == 401 || status == 403:
		out.ErrorCode = "auth_failed"
	case status == 429:
		out.ErrorCode = "rate_limited"
	default:
		out.ErrorCode, out.Message = "unavailable", "status "+strconv.Itoa(status)
	}
	return out.withResult()
}
