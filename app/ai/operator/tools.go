/*
|--------------------------------------------------------------------------
| Operator Tools
|--------------------------------------------------------------------------
|
| Operator acts only through tools. Reading tools are always offered;
| a tool that changes something is offered only while one of the groups
| it writes is not switched off. Each tool answers the model in text and
| the teammate with a one-line step, and a tool that is refused says so
| plainly instead of failing the run.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Step is what a teammate sees of one tool call: the tool, a short
summary, whether it worked, the reply it drafted and the changeset it
proposed.
*/
type Step struct {
	Tool      string `json:"tool"`
	Summary   string `json:"summary"`
	OK        bool   `json:"ok"`
	Draft     string `json:"draft,omitempty"`
	Changeset string `json:"changeset_id,omitempty"`
}

/*
result is what a tool returns: the text the model reads and the step
the teammate sees.
*/
type result struct {
	text string
	step Step
}

/*
handler runs one tool call.
*/
type handler func(ctx context.Context, s *session, args json.RawMessage) (result, error)

/*
tool is one tool: its spec, the operation kinds it may write, and its
handler.
*/
type tool struct {
	spec   contracts.ToolSpec
	writes []Kind
	handle handler
}

/*
registry returns every tool by name.
*/
func (e *Engine) registry(s *session) map[string]tool {
	all := slices.Concat(e.conversationTools(), e.knowledgeTools(), e.insightTools(), e.writeTools(), e.actionTools(s))
	out := make(map[string]tool, len(all))
	for _, t := range all {
		out[t.spec.Name] = t
	}
	return out
}

/*
toolSpecs returns the specs of the tools offered to this run, by name.
*/
func (e *Engine) toolSpecs(s *session) []contracts.ToolSpec {
	var out []contracts.ToolSpec
	for _, t := range e.registry(s) {
		if e.offers(s, t) {
			out = append(out, t.spec)
		}
	}
	slices.SortFunc(out, func(a, b contracts.ToolSpec) int {
		if a.Name < b.Name {
			return -1
		}
		return 1
	})
	return out
}

/*
offers reports whether a tool is offered under the settings: reading
tools always, writing tools while any group they write is on.
*/
func (e *Engine) offers(s *session, t tool) bool {
	if len(t.writes) == 0 {
		return true
	}
	for _, k := range t.writes {
		if offered(s.settings, k) {
			return true
		}
	}
	return false
}

/*
callTool runs one tool call, records it, streams its step and returns
what the model reads, saying what went wrong when it failed.
*/
func (e *Engine) callTool(ctx context.Context, s *session, call contracts.ToolCall) (string, bool) {
	started := e.Clock.Now()
	s.toolCalls++
	t, ok := e.registry(s)[call.Name]
	var out result
	err := errors.New("unknown tool " + call.Name)
	if ok && e.offers(s, t) {
		out, err = t.handle(ctx, s, call.Arguments)
	}
	if out.step.Tool == "" {
		out.step = Step{Tool: call.Name, Summary: call.Name}
	}
	out.step.OK = err == nil
	if err != nil {
		out.text, out.step.Summary = refusal(err), out.step.Summary+": "+refusal(err)
	}
	s.steps = append(s.steps, out.step)
	e.publishStep(ctx, s, out.step)
	e.record(ctx, s, step{
		kind: enums.AIRunStepKindToolCall, name: call.Name, started: started,
		request: string(call.Arguments), response: out.text, err: err,
	})
	return out.text, err != nil
}

/*
refusal says in plain words why a tool call did not work.
*/
func refusal(err error) string {
	if f, ok := errors.AsType[*fault.Error](err); ok {
		return f.Message()
	}
	switch {
	case errors.Is(err, exceptions.ErrForbidden):
		return "not allowed: the teammate's role does not permit this"
	case errors.Is(err, exceptions.ErrNotFound):
		return "not found, or not visible to this teammate"
	case errors.Is(err, errBadArguments):
		return err.Error()
	default:
		return "failed: " + err.Error()
	}
}

/*
errBadArguments marks a tool call whose arguments make no sense.
*/
var errBadArguments = errors.New("invalid arguments")

/*
decode reads a tool call's arguments into v.
*/
func decode(args json.RawMessage, v any) error {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if err := json.Unmarshal(args, v); err != nil {
		return errors.Join(errBadArguments, err)
	}
	return nil
}

/*
schema reads a tool's JSON schema, written as a constant.
*/
func schema(text string) json.RawMessage {
	return json.RawMessage(text)
}
