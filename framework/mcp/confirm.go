/*
|--------------------------------------------------------------------------
| Confirmation
|--------------------------------------------------------------------------
|
| Some tools act where a person must agree first: they reach customers,
| publish, spend money or send data out. An agent can be steered by text
| it reads, so its own say-so is not enough. Such a tool passes the
| application's confirmation hook before it runs, on every call and for
| every kind of credential.
|
| The hook decides. It can let the call run, ask the person through the
| client when the client can show a form, or answer with a result that
| tells the agent to get approval and call again. When the client asked
| the person, its retry carries their answer and the state the hook
| handed out, and the hook decides again. What makes a confirmation
| valid, such as sealing the state or remembering used ones, is the
| application's policy.
|
*/

package mcp

import (
	"context"
	"encoding/json"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

/*
Confirmer decides whether a call to a tool that needs confirmation runs.
It returns nil to run the tool, or what the caller gets instead. An
error fails the call as a tool error would.
*/
type Confirmer func(ctx context.Context, c Confirmation) (*Unconfirmed, error)

/*
Confirmation is a call waiting on the hook.

Tool is the tool called, Input its decoded input and Arguments the
arguments as sent. Answers holds the person's answers when the client
retries after asking them, keyed by the question's ID, and State the
state the hook handed out with the questions. CanAsk reports whether the
client can ask the person through a form.
*/
type Confirmation struct {
	Tool      string
	Input     any
	Arguments json.RawMessage
	Answers   map[string]Answer
	State     string
	CanAsk    bool
}

/*
Answer is a person's answer to a question. Action is "accept",
"decline" or "cancel", and Content holds the form's values when they
accepted.
*/
type Answer struct {
	Action  string
	Content map[string]any
}

/*
Unconfirmed is what a caller gets instead of the tool running.

When Ask is set and the client can ask, the client shows the question to
the person and calls again with their answer and State. Otherwise the
caller gets Message as text and Result as structured content, such as a
preview of what would happen and how to confirm it, or the news that the
person declined. The result is not an error.
*/
type Unconfirmed struct {
	Message string
	Result  any
	Ask     *Question
	State   string
}

/*
Question asks the person to confirm through a form. ID names the answer
in the retry, Message is shown to the person and Schema is the JSON
schema of the form, such as a single required boolean.
*/
type Question struct {
	ID      string
	Message string
	Schema  map[string]any
}

/*
newProtocol is the first protocol version in which a client answers
questions by calling again instead of being asked mid-call.
*/
const newProtocol = "2026-07-28"

/*
confirmation passes the call through the hook. It returns a nil result
and error when the tool may run.
*/
func (s *Server) confirmation(
	ctx context.Context, req *sdk.CallToolRequest, tool string, input any,
) (*sdk.CallToolResult, error) {
	if s.confirm == nil {
		return nil, nil
	}

	c := Confirmation{
		Tool:      tool,
		Input:     input,
		Arguments: req.Params.Arguments,
		Answers:   answers(req.Params.InputResponses),
		State:     req.Params.RequestState,
		CanAsk:    canAsk(req),
	}

	held, err := s.confirm(ctx, c)
	if err != nil {
		return s.failure(ctx, err)
	}

	if held == nil {
		return nil, nil
	}

	if held.Ask != nil && c.CanAsk {
		return &sdk.CallToolResult{
			InputRequests: sdk.InputRequestMap{held.Ask.ID: &sdk.ElicitParams{
				Mode:            "form",
				Message:         held.Ask.Message,
				RequestedSchema: held.Ask.Schema,
			}},
			RequestState: held.State,
		}, nil
	}

	return &sdk.CallToolResult{
		Content:           []sdk.Content{&sdk.TextContent{Text: held.Message}},
		StructuredContent: held.Result,
	}, nil
}

/*
answers collects the answers to form questions from a retried call.
*/
func answers(responses sdk.InputResponseMap) map[string]Answer {
	if len(responses) == 0 {
		return nil
	}

	out := map[string]Answer{}

	for id, response := range responses {
		if result, ok := response.(*sdk.ElicitResult); ok {
			out[id] = Answer{Action: result.Action, Content: result.Content}
		}
	}

	return out
}

/*
canAsk reports whether the client speaks a protocol in which it answers
questions by calling again, and can show a form. A client declaring
elicitation without naming a mode can show forms.
*/
func canAsk(req *sdk.CallToolRequest) bool {
	if req.ProtocolVersion() < newProtocol {
		return false
	}

	caps := req.ClientCapabilities()
	if caps == nil || caps.Elicitation == nil {
		return false
	}

	return caps.Elicitation.Form != nil || caps.Elicitation.URL == nil
}
