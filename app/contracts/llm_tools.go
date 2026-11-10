/*
|--------------------------------------------------------------------------
| LLM Tool Types
|--------------------------------------------------------------------------
|
| A tool is a named function the model may call; the contract only
| carries its description and the finished call — dispatch lives
| with the caller. A response schema is the typed answer a caller
| needs back, which the provider itself must enforce.
|
*/

package contracts

import "encoding/json"

/*
ToolSpec describes one tool the model may call. Names match the agent
tool rules so they stay valid for every provider.
*/
type ToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

/*
ToolCall is one invocation the model asked for.
*/
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

/*
ToolChoice tells the model how it may use tools.
*/
type ToolChoice string

/*
The tool choices. Forced tool use is never sent: some models reject it.
*/
const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

/*
ResponseSchema asks for a typed answer. When a request carries one, the
provider must return exactly one JSON object valid against Schema, and
the response's Text holds it. Name labels the schema for providers that
want one; it follows the tool naming rules. Schema is a JSON Schema
object; strict providers also need every object to close with
additionalProperties false and list each property as required.
*/
type ResponseSchema struct {
	Name   string
	Schema json.RawMessage
}
