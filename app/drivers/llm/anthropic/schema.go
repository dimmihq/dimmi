/*
|--------------------------------------------------------------------------
| Anthropic Response Schemas
|--------------------------------------------------------------------------
|
| A typed answer is one forced, strict tool call: the schema becomes
| the input schema of a single tool named after it, tool choice names
| that tool, and the tool's input is handed back as the answer's text.
| The caller never sees a tool call. A request that brings its own
| tools cannot also force one, so it is refused rather than guessed at.
|
*/

package anthropic

import (
	"encoding/json"

	anth "github.com/anthropics/anthropic-sdk-go"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
applySchema turns a response schema into the forced strict tool. It
refuses a request that also carries tools, an unnamed schema, or one
that is not a JSON object.
*/
func applySchema(params *anth.MessageNewParams, req contracts.ChatRequest) error {
	if len(req.Tools) > 0 {
		return schemaError("cannot be combined with tools")
	}
	if req.Schema.Name == "" {
		return schemaError("needs a name")
	}

	var schema map[string]any
	if err := json.Unmarshal(req.Schema.Schema, &schema); err != nil {
		return schemaError(req.Schema.Name + " is not a JSON object")
	}

	params.Tools = []anth.ToolUnionParam{{OfTool: &anth.ToolParam{
		Name:        req.Schema.Name,
		Description: anth.String("Give the answer. Call this exactly once."),
		Strict:      anth.Bool(true),
		InputSchema: anth.ToolInputSchemaParam{Properties: schema["properties"], ExtraFields: schema},
	}}}
	params.ToolChoice = anth.ToolChoiceUnionParam{OfTool: &anth.ToolChoiceToolParam{
		Name: req.Schema.Name, DisableParallelToolUse: anth.Bool(true),
	}}
	return nil
}

/*
schemaError is the bad-request error for a response schema the driver
refuses.
*/
func schemaError(problem string) error {
	return &contracts.LLMError{
		Kind:         contracts.LLMErrorInvalidRequest,
		ProviderCode: "invalid_response_schema",
		Message:      "response schema " + problem,
	}
}

/*
schemaName is the forced tool's name, empty when the request asks for
no typed answer.
*/
func schemaName(req contracts.ChatRequest) string {
	if req.Schema == nil {
		return ""
	}
	return req.Schema.Name
}

/*
asAnswer folds the forced tool call back into a typed answer: its
input becomes the text, the call disappears, and a reply that ended to
call it ended normally.
*/
func asAnswer(out *contracts.ChatResponse, answer string) {
	calls := out.ToolCalls
	out.Text, out.ToolCalls = "", nil
	for _, call := range calls {
		if call.Name == answer {
			out.Text = string(call.Arguments)
		}
	}
	if out.StopReason == contracts.StopReasonToolUse {
		out.StopReason = contracts.StopReasonEnd
	}
}
