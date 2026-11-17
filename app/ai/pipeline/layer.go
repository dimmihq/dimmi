/*
|--------------------------------------------------------------------------
| Tool Layer
|--------------------------------------------------------------------------
|
| With the agent's tool layer on, the model is offered search_knowledge,
| handoff, load_tools and call_tool, the same on every call. Every other
| tool the turn allows sits in a group named in the system prompt, and
| call_tool runs it only once loaded, with its arguments checked and a
| reading tool's result trimmed. The newest message names the tools the
| conversation loaded before, which this turn must load again, and the
| tools likeliest to help. What the layer did is kept on the run.
|
| The layer saves only on a model that caches the prompt: loading a
| tool costs a model call, which re-reads the whole prefix, and on the
| benchmark a model without a prompt cache spent more with the layer
| than without it. Such a model gets every tool as before.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
layerIndexMax is the most tool names a group's line lists in the
agent's prompt; the agent's groups are small, so they list them all.
*/
const layerIndexMax = 40

/*
Tool groups behind the layer.
*/
const (
	groupInbox      = "inbox"
	groupTickets    = "tickets"
	groupProcedures = "procedures"
	groupActions    = "actions"
)

/*
layerState is the layer of one run: the set, the tools the newest
message named, and the tools earlier turns loaded.
*/
type layerState struct {
	set       *toolset.Set
	suggested []string
	earlier   []string
	fixed     []string
}

/*
fixedTools are the tools offered outside the layer. The procedure tool
is one of them because its description is the list of the team's
procedures and when each applies: behind the layer the agent saw only
that procedures exist, so it could not tell a request matched one and
almost never followed any.
*/
func fixedTools() []string {
	return []string{toolSearchKnowledge, toolHandoff, toolProcedure}
}

/*
fixedFor is the turn's fixed list: the built-in fixed tools, and the
workspace's actions too when they are few and small enough to offer
directly.
*/
func (r *Runner) fixedFor(st *run, tools map[string]tool) []string {
	fixed := fixedTools()
	var names []string
	var specs []contracts.ToolSpec
	for name, t := range tools {
		if t.action && st.settings.allows(name, t.action) {
			names, specs = append(names, name), append(specs, t.spec)
		}
	}
	if toolset.Inline(specs) {
		fixed = append(fixed, names...)
	}
	slices.Sort(fixed)
	return fixed
}

/*
layerGroups are the groups, one line each.
*/
func layerGroups() []toolset.Group {
	return []toolset.Group{
		{Name: groupInbox, Line: "tag, assign and prioritize this conversation"},
		{Name: groupTickets, Line: "open a ticket the team follows up"},
		{Name: groupProcedures, Line: "step-by-step procedures the team wrote for set cases"},
		{Name: groupActions, Line: "look things up and make changes in the company's own systems"},
	}
}

/*
groupOf names a tool's group.
*/
func groupOf(name string, t tool) string {
	switch {
	case t.action:
		return groupActions
	case name == toolOpenTicket:
		return groupTickets
	case name == toolProcedure:
		return groupProcedures
	default:
		return groupInbox
	}
}

/*
openLayer builds the run's layer when the agent has it on: the tools
behind it, the groups in the system prompt, and the note on the newest
message. It runs once the turn's actions are offered.
*/
func (r *Runner) openLayer(ctx context.Context, st *run) {
	if !st.env.Agent.ToolLayer || st.loop == nil || len(st.loop.messages) == 0 ||
		!st.loop.llm.Capabilities(strv(st.env.Agent.AnswerModel)).PromptCache {
		return
	}
	tools := r.registry(st)
	fixed := r.fixedFor(st, tools)
	var entries []toolset.Entry
	for name, t := range tools {
		if slices.Contains(fixed, name) || !st.settings.allows(name, t.action) {
			continue
		}
		entries = append(entries, toolset.Entry{
			Spec: t.spec, Group: groupOf(name, t), Reads: t.effect == sideEffectRead,
		})
	}
	if len(entries) == 0 {
		return
	}
	earlier := r.earlierLoads(ctx, st, fixed)
	set := toolset.New(entries, layerGroups(), earlier).WithIndexMax(layerIndexMax).WithFixed(fixed)
	if st.env.Agent.CompactResults {
		set.WithCompact()
	}
	ls := &layerState{set: set, earlier: earlier, fixed: fixed}
	ls.suggested = set.Suggest(ctx, toolset.Keywords{}, r.redactFor(st.env, st.env.pendingText()))
	st.layer = ls

	lp := st.loop
	lp.system += "\n\n" + set.Prompt()
	last := &lp.messages[len(lp.messages)-1]
	if note := toolset.Note(earlier, ls.suggested); note != "" {
		last.Parts = append(last.Parts, contracts.ContentPart{Text: note})
	}
	if len(lp.messages) > 1 {
		lp.messages[len(lp.messages)-2].Cacheable = true
	}
}

/*
earlierLoads are the tools the conversation's earlier runs loaded that
the layer still holds.
*/
func (r *Runner) earlierLoads(ctx context.Context, st *run, fixed []string) []string {
	if st.rec == nil || st.env.Conversation.ID == (ids.UUID{}) {
		return nil
	}
	var records []json.RawMessage
	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		records, err = models.New(tx).ListConversationToolLayers(ctx, models.ListConversationToolLayersParams{
			WorkspaceID: st.env.Workspace.ID, ConversationID: &st.env.Conversation.ID, Before: st.rec.Run.ID,
		})
		return err
	})
	registry := r.registry(st)
	return toolset.Earlier(records, func(name string) bool {
		t, ok := registry[name]
		return ok && st.settings.allows(name, t.action) && !slices.Contains(fixed, name)
	})
}

/*
layerSpecs are the specs offered with the layer on: the fixed tools the
turn allows and the layer's own.
*/
func (r *Runner) layerSpecs(st *run) []contracts.ToolSpec {
	var out []contracts.ToolSpec
	tools := r.registry(st)
	for _, name := range st.layer.fixed {
		if t, ok := tools[name]; ok && st.settings.allows(name, t.action) {
			out = append(out, t.spec)
		}
	}
	out = append(out, toolset.Specs()...)
	slices.SortFunc(out, func(a, b contracts.ToolSpec) int {
		if a.Name < b.Name {
			return -1
		}
		return 1
	})
	return out
}

/*
layerCall runs one of the layer's own tools and returns the tool the
call ran, its result and its error.
*/
func (r *Runner) layerCall(ctx context.Context, st *run, call contracts.ToolCall) (string, string, error) {
	set := st.layer.set
	if call.Name == toolset.LoadTool {
		out, err := set.Load(call.Arguments)
		return call.Name, out, err
	}
	if inner := innerName(call.Arguments); set.IsFixed(inner) {
		return r.runFixed(ctx, st, inner, call.Arguments)
	}
	c, err := set.Check(call.Arguments)
	if err != nil {
		return call.Name, "", err
	}
	if c.Name == toolset.ReadRest {
		out, err := set.Rest(c.Arguments)
		return c.Name, out, err
	}
	t, ok := r.registry(st)[c.Name]
	if !ok || !st.settings.allows(c.Name, t.action) {
		return c.Name, "", errors.New("tool " + c.Name + " is not available in this turn")
	}
	out, err := t.handle(ctx, st, c.Arguments)
	if err == nil && t.effect == sideEffectRead {
		out = set.Shape(out, c.Fields)
	}
	if err == nil {
		out = set.Compact(out)
	}
	return c.Name, out, err
}

/*
runFixed runs a fixed tool the model reached through call_tool, as if
it had called it by name.
*/
func (r *Runner) runFixed(ctx context.Context, st *run, name string, args json.RawMessage) (string, string, error) {
	var in struct {
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(args, &in)
	t, ok := r.registry(st)[name]
	if !ok || !st.settings.allows(name, t.action) {
		return name, "", errors.New("tool " + name + " is not available in this turn")
	}
	if len(in.Arguments) == 0 {
		in.Arguments = json.RawMessage("{}")
	}
	out, err := t.handle(ctx, st, in.Arguments)
	return name, out, err
}

/*
innerName is the tool a call_tool request names.
*/
func innerName(args json.RawMessage) string {
	var in struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(args, &in)
	return in.Name
}

/*
saveLayer keeps what the layer did on the run.
*/
func (r *Runner) saveLayer(ctx context.Context, st *run) {
	if st.layer == nil || st.rec == nil {
		return
	}
	body, err := json.Marshal(st.layer.set.Record(r.layerSpecs(st), st.layer.suggested, st.layer.earlier,
		st.loop.system))
	if err != nil {
		return
	}
	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SetRunToolLayer(ctx, models.SetRunToolLayerParams{
			ToolLayer: body, WorkspaceID: st.rec.Run.WorkspaceID, ID: st.rec.Run.ID,
		})
	})
}
