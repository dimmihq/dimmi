/*
|--------------------------------------------------------------------------
| LLM Tool Types
|--------------------------------------------------------------------------
|
| A tool is a named function the model may call; the contract only
| carries its description and the finished call — dispatch lives
| with the caller.
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
ResponseFormat names the answer shape the caller needs.
*/
type ResponseFormat string

/*
The answer shapes. JSON is used by the classifier; drivers map it to
the native mode or a prompt suffix where none exists.
*/
const (
	ResponseFormatText ResponseFormat = "text"
	ResponseFormatJSON ResponseFormat = "json"
)
