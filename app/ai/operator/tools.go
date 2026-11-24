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
| The MCP server offers the same work under the same names with a
| dimmi_ prefix. The tools below stay Operator's own, each for a reason
| the MCP tool cannot meet inside a run:
|
|   - The tools that read conversations, customers, AI runs, reports
|     and GitHub answer in trimmed text a run can afford, and mark the
|     run as having read what a customer wrote.
|   - list_articles pages every help center by offset, which a
|     routine's saved state counts on; the MCP lists one help center.
|   - The propose tools, save_routine_state and
|     report_nothing_to_change write to the run itself: its thread, its
|     outcome and whether it is a dry run.
|   - draft_reply and remember belong to the teammate's thread.
|   - The workspace's actions run with the conversation's customer.
|
| search_knowledge, list_recent_suggestions, get_skill_by_name and the
| suggestion checks are one implementation, which the MCP tools call.
| A chat reads the rest of the MCP catalog through the tool layer; a
| routine is offered a few of its reads by name, as its narrowed owner.
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
	Subagent  int    `json:"subagent,omitempty"`
}

/*
result is what a tool returns: the text the model reads and the step
the teammate sees.
*/
type result struct {
	text string
	step Step
	/* A tool that added its own steps to the run's timeline asks for none to be added for the call. */
	quiet bool
}

/*
handler runs one tool call.
*/
type handler func(ctx context.Context, s *session, args json.RawMessage) (result, error)

/*
tool is one tool: its spec, the operation kinds it may write, and its
handler. Routine marks a tool only a routine's run is offered; group
names the routine tool group it belongs to when that is not the group
the layer files it under.
*/
type tool struct {
	spec    contracts.ToolSpec
	writes  []Kind
	handle  handler
	routine bool
	group   string
	was     map[string]string
	/* Whether subagents are kept from the tool, and whether it runs with no other call beside it. */
	mainOnly, alone bool
}

/*
registry returns every tool by name.
*/
func (e *Engine) registry(s *session) map[string]tool {
	all := slices.Concat(e.conversationTools(), e.knowledgeTools(), e.insightTools(), e.procedureTools(),
		e.writeTools(), e.actionTools(s), e.routineTools(), e.githubTools(),
		[]tool{e.subagentsTool(), e.suggestionTool(), e.relatedTool()}, e.catalogTools(s))
	out := make(map[string]tool, len(all))
	for _, t := range all {
		if handle, was := t.handle, t.was; len(was) > 0 {
			t.handle = func(ctx context.Context, s *session, args json.RawMessage) (result, error) {
				return handle(ctx, s, renamed(args, was))
			}
		}
		out[t.spec.Name] = t
	}
	return out
}

/*
toolSpecs returns the specs of the tools offered to this run, by name.
*/
func (e *Engine) toolSpecs(s *session) []contracts.ToolSpec {
	if s.layer != nil {
		return e.layerSpecs(s)
	}
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
tools always, writing tools while any group they write is on. A routine
is offered only the groups it was given, never drafts or memories, and
the routine tools only to a routine.
*/
func (e *Engine) offers(s *session, t tool) bool {
	if s.routine == nil && t.routine {
		return false
	}
	if s.sub != nil && t.mainOnly {
		return false
	}
	if s.routine != nil && !s.routine.allows(e, t) {
		return false
	}
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
refusal says in plain words why a tool call did not work.
*/
func refusal(err error) string {
	if errors.Is(err, errSpent) {
		return err.Error()
	}
	if errors.Is(err, exceptions.ErrNotFound) {
		return "not found, or not visible to this teammate: check the ids you sent"
	}
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
errSpent refuses a reading tool once a routine's run may only close.
*/
var errSpent = errors.New("you are out of reading steps for this run, and every further reading call will be " +
	"refused the same way. Stop reading now. Suggest what you already found with propose_article_edits, " +
	"propose_new_article or propose_changes, save progress with save_routine_state, or call " +
	"report_nothing_to_change; then write your message")

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
