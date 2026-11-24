/*
|--------------------------------------------------------------------------
| Tool Layer
|--------------------------------------------------------------------------
|
| With the layer on, Operator is offered search_knowledge,
| get_conversation_by_number and search_conversations, plus load_tools and
| call_tool, the same on every call. Its other tools, and the read-only
| tools of the API catalog, sit in groups named in the system prompt:
| call_tool runs one once it is loaded, with its arguments checked and a
| reading tool's result trimmed. The layer never widens what Operator
| may do: its own tools keep their write policies and changesets, and
| the catalog is reached only to read, as the teammate. A catalog read
| may hold what a customer wrote, so a run that made one applies
| nothing on its own afterwards. A model without
| a prompt cache gets every tool instead: each load re-reads the prefix
| at full price, which cost more than the layer saved on the benchmark.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Catalog is the API catalog's read-only tools: their definitions, and a
call made as the teammate.
*/
type Catalog interface {
	/*
		Tools are the catalog's read-only tools.
	*/
	Tools() []CatalogTool

	/*
		Call runs one as the teammate.
	*/
	Call(ctx context.Context, m authz.Membership, call CatalogCall) (string, error)
}

/*
CatalogCall is one catalog read: the tool, its arguments, and whether
to return the API's JSON body for the layer to trim instead of text.
*/
type CatalogCall struct {
	Name string
	Args json.RawMessage
	Raw  bool
}

/*
CatalogTool is one read-only catalog tool and its group.
*/
type CatalogTool struct {
	Spec  contracts.ToolSpec
	Group string
	Line  string
}

/*
Operator's own groups.
*/
const (
	groupConversations = "conversations"
	groupKnowledge     = "knowledge"
	groupInsights      = "insights"
	groupChanges       = "changes"
	groupActions       = "actions"
	groupAutomation    = "automation"
)

/*
layerState is the layer of one run.
*/
type layerState struct {
	set       *toolset.Set
	suggested []string
	earlier   []string
	catalog   map[string]bool
	system    string
	fixed     []string
}

/*
fixedTools are the tools offered outside the layer.
*/
func fixedTools() []string {
	return []string{"get_conversation_by_number", "search_conversations", "search_knowledge"}
}

/*
fixedFor is the run's fixed list: the built-in fixed tools, and the
actions offered to Operator too when they are few and small enough to
offer directly.
*/
func (e *Engine) fixedFor(s *session, registry map[string]tool) []string {
	fixed := fixedTools()
	var names []string
	var specs []contracts.ToolSpec
	for name, t := range registry {
		if strings.HasPrefix(name, actionPrefix) && e.offers(s, t) {
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
ownGroups are Operator's own groups, one line each.
*/
func ownGroups() []toolset.Group {
	return []toolset.Group{
		{Name: groupConversations, Line: "list conversations, read customers, tags and teammates"},
		{Name: groupKnowledge, Line: "help center articles and procedures"},
		{Name: groupInsights, Line: "workspace reports and the AI agent's past runs"},
		{Name: groupChanges, Line: "draft replies, propose changes the teammate applies, remember notes"},
		{Name: groupActions, Line: "the workspace's own actions on its systems"},
		{Name: groupAutomation, Line: "workflows and series: read them, explain what workflows did on a " +
			"conversation, suggest edits and new drafts"},
	}
}

/*
ownGroupOf names the group of one of Operator's own tools.
*/
func ownGroupOf(t tool) string {
	switch {
	case strings.HasPrefix(t.spec.Name, actionPrefix):
		return groupActions
	case strings.Contains(t.spec.Name, "workflow") || strings.Contains(t.spec.Name, "series"):
		return groupAutomation
	case len(t.writes) > 0 || t.spec.Name == "remember" || t.spec.Name == "draft_reply":
		return groupChanges
	case t.spec.Name == "get_workspace_report" || t.spec.Name == "get_ai_agent_runs":
		return groupInsights
	case strings.Contains(t.spec.Name, "article") || strings.Contains(t.spec.Name, "procedure"):
		return groupKnowledge
	default:
		return groupConversations
	}
}

/*
openLayer builds the run's layer when the settings have it on.
*/
func (e *Engine) openLayer(ctx context.Context, s *session) {
	if !s.settings.ToolLayer || !s.llm.Capabilities(s.model).PromptCache {
		return
	}
	registry := e.registry(s)
	fixed := e.fixedFor(s, registry)
	var entries []toolset.Entry
	for name, t := range registry {
		if slices.Contains(fixed, name) || !e.offers(s, t) {
			continue
		}
		entries = append(entries, toolset.Entry{Spec: t.spec, Group: ownGroupOf(t), Reads: len(t.writes) == 0 &&
			ownGroupOf(t) != groupChanges})
	}
	groups := ownGroups()
	ls := &layerState{catalog: map[string]bool{}, fixed: fixed}
	if e.Catalog != nil && s.routine == nil {
		seen := map[string]bool{}
		for _, c := range e.Catalog.Tools() {
			entries = append(entries, toolset.Entry{Spec: c.Spec, Group: c.Group, Reads: true})
			ls.catalog[c.Spec.Name] = true
			if !seen[c.Group] {
				seen[c.Group] = true
				groups = append(groups, toolset.Group{Name: c.Group, Line: c.Line})
			}
		}
	}
	ls.earlier = e.earlierLoads(ctx, s, func(n string) bool {
		return slices.ContainsFunc(entries,
			func(en toolset.Entry) bool { return en.Spec.Name == n })
	})
	ls.set = toolset.New(entries, groups, ls.earlier).WithFixed(fixed)
	if s.settings.CompactResults {
		ls.set.WithCompact()
	}
	ls.suggested = ls.set.Suggest(ctx, toolset.Keywords{}, s.question)
	s.layer = ls
}

/*
earlierLoads are the tools the thread's earlier runs loaded.
*/
func (e *Engine) earlierLoads(ctx context.Context, s *session, known func(string) bool) []string {
	var runs []ids.UUID
	for _, m := range s.history {
		if m.RunID != nil {
			runs = append(runs, *m.RunID)
		}
	}
	if len(runs) == 0 {
		return nil
	}
	var records []json.RawMessage
	_ = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		records, err = models.New(tx).ListRunToolLayers(ctx, models.ListRunToolLayersParams{
			WorkspaceID: s.m.Workspace.ID, Ids: runs,
		})
		return err
	})
	return toolset.Earlier(records, known)
}

/*
layerSpecs are the specs offered with the layer on.
*/
func (e *Engine) layerSpecs(s *session) []contracts.ToolSpec {
	var out []contracts.ToolSpec
	registry := e.registry(s)
	for _, name := range s.layer.fixed {
		if t, ok := registry[name]; ok && e.offers(s, t) {
			out = append(out, t.spec)
		}
	}
	out = append(out, toolset.Specs()...)
	slices.SortFunc(out, func(a, b contracts.ToolSpec) int { return strings.Compare(a.Name, b.Name) })
	return out
}

/*
innerName is the tool a call_tool request names, read before the call
is checked so a writing tool runs alone.
*/
func innerName(args json.RawMessage) string {
	var in struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(args, &in)
	return in.Name
}

/*
layerCall runs one of the layer's own tools.
*/
func (e *Engine) layerCall(ctx context.Context, s *session, p *pending) (result, error) {
	set := s.layer.set
	if p.call.Name == toolset.LoadTool {
		out, err := set.Load(p.call.Arguments)
		return result{text: out, step: Step{Tool: toolset.LoadTool, Summary: "Looked up tools"}}, err
	}
	if inner := innerName(p.call.Arguments); set.IsFixed(inner) {
		return e.runFixed(ctx, s, p, inner)
	}
	c, err := set.Check(p.call.Arguments)
	if err != nil {
		return result{}, errors.Join(errBadArguments, err)
	}
	p.ran = c.Name
	if c.Name == toolset.ReadRest {
		out, err := set.Rest(c.Arguments)
		return result{text: out, step: Step{Tool: c.Name, Summary: "Read on"}}, err
	}
	if s.layer.catalog[c.Name] {
		out, err := e.Catalog.Call(ctx, s.m, CatalogCall{Name: c.Name, Args: c.Arguments, Raw: len(c.Fields) > 0})
		if err != nil {
			return result{}, err
		}
		s.tainted = true
		return result{text: set.Shape(out, c.Fields), step: Step{Tool: c.Name, Summary: "Read " +
			strings.TrimPrefix(c.Name, "dimmi_")}}, nil
	}
	t, ok := e.registry(s)[c.Name]
	if !ok || !e.offers(s, t) {
		return result{}, errors.New("tool " + c.Name + " is not available")
	}
	out, err := t.handle(ctx, s, c.Arguments)
	if err == nil && len(t.writes) == 0 && ownGroupOf(t) != groupChanges {
		out.text = set.Shape(out.text, c.Fields)
	}
	return out, err
}

/*
runFixed runs a fixed tool the model reached through call_tool, as if
it had called it by name.
*/
func (e *Engine) runFixed(ctx context.Context, s *session, p *pending, name string) (result, error) {
	p.ran = name
	if !p.known {
		return result{}, errors.New("tool " + name + " is not available")
	}
	var in struct {
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(p.call.Arguments, &in)
	args, err := s.layer.set.Expand(in.Arguments)
	if err != nil {
		return result{}, errors.Join(errBadArguments, err)
	}
	return p.tool.handle(ctx, s, args)
}

/*
saveLayer keeps what the layer did on the run.
*/
func (e *Engine) saveLayer(ctx context.Context, s *session) {
	if s.layer == nil || s.run.ID == (ids.UUID{}) {
		return
	}
	body, err := json.Marshal(s.layer.set.Record(e.layerSpecs(s), s.layer.suggested, s.layer.earlier, s.layer.system))
	if err != nil {
		return
	}
	_ = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SetRunToolLayer(ctx, models.SetRunToolLayerParams{
			ToolLayer: body, WorkspaceID: s.run.WorkspaceID, ID: s.run.ID,
		})
	})
}
