/*
|--------------------------------------------------------------------------
| Tool Layer
|--------------------------------------------------------------------------
|
| A model reads every tool definition on every call, so an agent with
| many tools pays for all of them each time. The layer offers a fixed,
| short list instead: the few tools nearly every turn needs, plus
| load_tools and call_tool. Every other tool sits in a group named by
| one line of the system prompt. load_tools returns a tool's definition
| as a tool result, and call_tool runs it after checking its arguments
| against its schema, answering in a short sentence the model can fix.
|
| The fixed list never changes, so the tool definitions and system
| prompt stay one cacheable prefix. A tool loaded during a turn stays
| loaded until the next customer or teammate message, when its
| definition leaves the history with the rest of the turn's tool
| results; the newest message then names the tools to reload.
|
*/

package toolset

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/actionschema"
)

/*
The layer's own tools.
*/
const (
	LoadTool = "load_tools"
	CallTool = "call_tool"
	ReadRest = "read_rest"
)

/*
fieldsArgument is the optional argument of a reading tool that keeps
only the named fields of its result.
*/
const fieldsArgument = "fields"

/*
groupIndexMax is the most tool names a group's line lists; a larger
group gives its count, and loading the group lists its tools.
*/
const groupIndexMax = 8

/*
InlineTokens is the most the definitions of a workspace's actions for
an agent may total, in estimated tokens, to stay in the fixed tool list
instead of behind load_tools. Below it, a load would cost a model call
for every action use and save little: about 2,000 tokens is what a
handful of actions with short schemas take, and under a tenth of the
system prompt. The fixed list then changes only when the workspace
changes its actions, so it still caches.
*/
const InlineTokens = 2000

/*
Inline reports whether specs are few and small enough to offer
directly.
*/
func Inline(specs []contracts.ToolSpec) bool {
	return len(specs) > 0 && Tokens(specs) <= InlineTokens
}

/*
Errors the layer answers with.
*/
var (
	ErrUnknownTool = errors.New("unknown tool")
	ErrArguments   = errors.New("invalid arguments")
)

/*
Entry is one tool behind the layer: its spec, the group it is listed
under, and whether it only reads, which lets its result be trimmed to
named fields.
*/
type Entry struct {
	Spec  contracts.ToolSpec
	Group string
	Reads bool
}

/*
Group is one group of tools and the line that says what it is for.
*/
type Group struct {
	Name string
	Line string
}

/*
Stats count what the layer did during one run: definitions loaded,
loads of tools named in the reload note, calls, calls refused for their
arguments, names the model asked for that do not exist, and the tools
it ran.
*/
type Stats struct {
	Loads     int      `json:"loads"`
	Reloads   int      `json:"reloads"`
	Calls     int      `json:"calls"`
	ArgErrors int      `json:"argument_errors"`
	Direct    int      `json:"direct_calls"`
	NotFound  []string `json:"not_found,omitempty"`
	Used      []string `json:"used,omitempty"`
}

/*
Set is the layer for one run: the tools behind it, their groups, what
is loaded, the cut texts read_rest can return, and the run's counts. A
set is used by one run; its methods are safe for the parallel reads a
run makes.
*/
type Set struct {
	mu      sync.Mutex
	entries map[string]*entry
	names   []string
	groups  []Group
	loaded  map[string]bool
	earlier map[string]bool
	cuts    []string
	stats   Stats
	limits  Limits
	listMax int
	fixed   map[string]bool
	alias   *aliases
}

/*
entry is an entry with its compiled schema, built on first use.
*/
type entry struct {
	Entry
	schema   *actionschema.Schema
	unusable bool
}

/*
New builds the layer over the entries and their groups, with the tools
loaded during the conversation's previous turns, so loading one again
counts as a reload.
*/
func New(entries []Entry, groups []Group, earlier []string) *Set {
	s := &Set{
		entries: map[string]*entry{}, groups: groups, loaded: map[string]bool{}, earlier: map[string]bool{},
		limits: DefaultLimits(), listMax: groupIndexMax,
	}
	for _, e := range entries {
		s.entries[e.Spec.Name] = &entry{Entry: e}
		s.names = append(s.names, e.Spec.Name)
	}
	slices.Sort(s.names)
	for _, n := range earlier {
		s.earlier[n] = true
	}
	return s
}

/*
WithLimits sets how far results are trimmed.
*/
func (s *Set) WithLimits(l Limits) *Set {
	s.limits = l
	return s
}

/*
WithFixed names the tools offered outside the layer, so loading one
answers that it is already offered instead of not found.
*/
func (s *Set) WithFixed(names []string) *Set {
	s.fixed = map[string]bool{}
	for _, n := range names {
		s.fixed[n] = true
	}
	return s
}

/*
IsFixed reports whether a tool is offered outside the layer.
*/
func (s *Set) IsFixed(name string) bool {
	return s.fixed[name]
}

/*
WithIndexMax sets the most tool names a group's line lists.
*/
func (s *Set) WithIndexMax(n int) *Set {
	s.listMax = n
	return s
}

/*
Has reports whether a tool is behind the layer.
*/
func (s *Set) Has(name string) bool {
	_, ok := s.entries[name]
	return ok
}

/*
Misdirected is the answer to a call that names a tool behind the layer
directly: such a tool runs only through call_tool, so the answer says
nothing was done and how to run it, loaded or not.
*/
func (s *Set) Misdirected(name string) string {
	s.mu.Lock()
	loaded := s.loaded[name]
	s.mu.Unlock()
	if loaded {
		return name + " is loaded, but it is not called by name. Nothing was done. Run it with " + CallTool +
			`, passing {"name": "` + name + `", "arguments": {...}}.`
	}
	return name + " is not called by name. Nothing was done. Load it with " + LoadTool + ", then run it with " +
		CallTool + `, passing {"name": "` + name + `", "arguments": {...}}.`
}

/*
Entries returns the entries, by name.
*/
func (s *Set) Entries() []Entry {
	out := make([]Entry, 0, len(s.names))
	for _, n := range s.names {
		out = append(out, s.entries[n].Entry)
	}
	return out
}

/*
Loaded returns the tools loaded so far, by name.
*/
func (s *Set) Loaded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.loaded))
	for n := range s.loaded {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

/*
Stats returns the run's counts.
*/
func (s *Set) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.stats
	out.NotFound = slices.Clone(out.NotFound)
	out.Used = slices.Clone(out.Used)
	return out
}

/*
Specs are the layer's own tool definitions, the same on every call.
*/
func Specs() []contracts.ToolSpec {
	return []contracts.ToolSpec{
		{
			Name: LoadTool,
			Description: "Load tools from the groups in your instructions. Pass names to get their full " +
				"definitions, or a group to list its tools. Then run a tool with call_tool.",
			Schema: json.RawMessage(`{"type":"object","properties":{"names":{"type":"array","items":` +
				`{"type":"string"},"description":"Tool names to load."},"group":{"type":"string",` +
				`"description":"A group to list."}},"additionalProperties":false}`),
		},
		{
			Name: CallTool,
			Description: "Run a tool from the groups. Load it first unless you know its arguments; " +
				"arguments that do not fit are refused with the tool's definition. To read the rest of a cut text, " +
				"run read_rest with its ref.",
			Schema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},` +
				`"arguments":{"type":"object"}},"required":["name"],"additionalProperties":false}`),
		},
	}
}

/*
Prompt is the instructions' list of groups, one line each, the tool
names of a small group included.
*/
func (s *Set) Prompt() string {
	members := map[string][]string{}
	for _, n := range s.names {
		g := s.entries[n].Group
		members[g] = append(members[g], n)
	}
	var b strings.Builder
	b.WriteString("More tools are in these groups. They are never called by name: load what you need with " +
		"load_tools, then run it with call_tool, passing the tool's name and arguments. Loaded tools last until " +
		"the next message.\n")
	for _, g := range s.groups {
		names := members[g.Name]
		if len(names) == 0 {
			continue
		}
		b.WriteString("- " + g.Name + ": " + g.Line)
		if len(names) <= s.listMax {
			b.WriteString(" (" + strings.Join(names, ", ") + ")")
		} else {
			fmt.Fprintf(&b, " (%d tools)", len(names))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

/*
definition is a tool's definition as load_tools returns it.
*/
type definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input_schema"`
}

/*
Load answers a load_tools call: the definitions of the named tools, or
the tools of a group, one line each.
*/
func (s *Set) Load(args json.RawMessage) (string, error) {
	var in struct {
		Names []string `json:"names"`
		Group string   `json:"group"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return "", err
	}
	if len(in.Names) == 0 && in.Group == "" {
		return "", fmt.Errorf("%w: pass names or a group", ErrArguments)
	}
	if in.Group != "" && len(in.Names) == 0 {
		return s.index(in.Group)
	}
	return s.define(in.Names)
}

/*
index lists a group's tools, one line each.
*/
func (s *Set) index(group string) (string, error) {
	var lines []string
	for _, n := range s.names {
		if s.entries[n].Group == group {
			lines = append(lines, n+": "+firstSentence(s.entries[n].Spec.Description))
		}
	}
	if len(lines) == 0 {
		s.miss(group)
		return "", fmt.Errorf("%w: no group %q; groups are %s", ErrUnknownTool, group, s.groupNames())
	}
	return strings.Join(lines, "\n"), nil
}

/*
define returns the named tools' definitions and marks them loaded.
*/
func (s *Set) define(names []string) (string, error) {
	var defs []definition
	var missing, offered []string
	for _, n := range names {
		e, ok := s.entries[n]
		if !ok && (s.fixed[n] || n == ReadRest) {
			offered = append(offered, n)
			continue
		}
		if !ok {
			missing = append(missing, n)
			continue
		}
		defs = append(defs, definition{Name: n, Description: e.Spec.Description, Input: inputOf(e.Entry)})
		s.markLoaded(n)
	}
	for _, n := range missing {
		s.miss(n)
	}
	out := map[string]any{"tools": defs}
	if len(defs) > 0 {
		out["how_to_run"] = "These tools are not called by name: run each with " + CallTool +
			", passing its name and arguments."
	}
	if len(offered) > 0 {
		out["already_offered"] = offered
	}
	if len(missing) > 0 {
		out["not_found"] = missing
		out["hint"] = "no such tools; load a group to list its tools: " + s.groupNames()
	}
	if len(defs) == 0 && len(offered) == 0 {
		return "", fmt.Errorf("%w: %s; load a group to list its tools: %s", ErrUnknownTool,
			strings.Join(missing, ", "), s.groupNames())
	}
	body, err := json.Marshal(out)
	return string(body), err
}

/*
markLoaded marks a tool loaded and counts it.
*/
func (s *Set) markLoaded(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded[name] = true
	s.stats.Loads++
	if s.earlier[name] {
		s.stats.Reloads++
	}
}

/*
miss records a name the model asked for that does not exist.
*/
func (s *Set) miss(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.NotFound = append(s.stats.NotFound, name)
}

/*
Call is a checked call_tool request: the tool, its arguments without
the layer's own, and the fields to keep of its result.
*/
type Call struct {
	Name      string
	Arguments json.RawMessage
	Fields    []string
}

/*
Check reads a call_tool request and checks it: the tool exists, is
loaded, and its arguments match its schema. A refusal is one short
sentence the model can act on.
*/
func (s *Set) Check(args json.RawMessage) (Call, error) {
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return Call{}, s.refuse(err)
	}
	if in.Name == ReadRest {
		return Call{Name: ReadRest, Arguments: orEmpty(in.Arguments)}, nil
	}
	e, ok := s.entries[in.Name]
	if !ok {
		s.miss(in.Name)
		return Call{}, s.refuse(fmt.Errorf("%w %q; load a group to list its tools: %s", ErrUnknownTool, in.Name,
			s.groupNames()))
	}
	c := Call{Name: in.Name, Arguments: orEmpty(in.Arguments)}
	if e.Reads {
		c.Arguments, c.Fields = splitFields(c.Arguments)
	}
	expanded, err := s.Expand(c.Arguments)
	if err != nil {
		return Call{}, s.refuse(err)
	}
	c.Arguments = expanded
	if err := s.admit(e, c.Arguments); err != nil {
		return Call{}, err
	}
	s.mu.Lock()
	s.stats.Calls++
	s.stats.Used = append(s.stats.Used, in.Name)
	s.mu.Unlock()
	return c, nil
}

/*
admit checks a call's arguments. A tool the model never loaded runs
when they fit and counts as loaded; when they do not, the refusal
carries its definition, so the model can fix the call without a load.
*/
func (s *Set) admit(e *entry, args json.RawMessage) error {
	s.mu.Lock()
	loaded := s.loaded[e.Spec.Name]
	s.mu.Unlock()
	if err := s.validate(e, args); err != nil {
		if loaded {
			return s.refuse(err)
		}
		s.markLoaded(e.Spec.Name)
		return s.refuse(fmt.Errorf("%w; its definition: %s", err, mustJSON(definition{
			Name: e.Spec.Name, Description: e.Spec.Description, Input: inputOf(e.Entry),
		})))
	}
	if !loaded {
		s.mu.Lock()
		s.loaded[e.Spec.Name] = true
		s.stats.Direct++
		s.mu.Unlock()
	}
	return nil
}

/*
refuse counts a refused call_tool request.
*/
func (s *Set) refuse(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.ArgErrors++
	return err
}

/*
validate checks arguments against the tool's schema, compiling it on
first use. A schema that cannot be compiled is left unchecked, as the
handler still checks what it reads.
*/
func (s *Set) validate(e *entry, args json.RawMessage) error {
	schema := s.schemaOf(e)
	if schema == nil {
		return nil
	}
	value := map[string]any{}
	if err := json.Unmarshal(args, &value); err != nil {
		return fmt.Errorf("%w: arguments must be a JSON object", ErrArguments)
	}
	if err := schema.Check(value); err != nil {
		return fmt.Errorf("%w for %s: %s", ErrArguments, e.Spec.Name, shortReason(err))
	}
	return nil
}

/*
schemaOf is a tool's compiled schema, nil when it cannot be compiled.
*/
func (s *Set) schemaOf(e *entry) *actionschema.Schema {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.schema == nil && !e.unusable && len(e.Spec.Schema) > 0 {
		compiled, err := actionschema.CompileExact(e.Spec.Schema)
		e.schema, e.unusable = compiled, err != nil
	}
	return e.schema
}

/*
shortReason is the first line of a validation error, cut to a length a
model reads at a glance.
*/
func shortReason(err error) string {
	text := err.Error()
	if i := strings.IndexByte(text, '\n'); i > 0 {
		text = text[:i]
	}
	text = strings.TrimPrefix(text, actionschema.ErrInvalidArguments.Error()+": ")
	text = strings.TrimPrefix(text, "validating root: ")
	const most = 240
	if len(text) > most {
		text = text[:most] + "…"
	}
	return text
}

/*
inputOf is a tool's schema as loaded: a reading tool's gains the
optional fields argument.
*/
func inputOf(e Entry) json.RawMessage {
	if !e.Reads {
		return e.Spec.Schema
	}
	root := map[string]any{}
	if json.Unmarshal(e.Spec.Schema, &root) != nil {
		return e.Spec.Schema
	}
	props, _ := root["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	if _, taken := props[fieldsArgument]; taken {
		return e.Spec.Schema
	}
	props[fieldsArgument] = map[string]any{
		"type": "array", "items": map[string]any{"type": "string"},
		"description": "Optional: keep only these fields of each result, such as id and title.",
	}
	root["properties"] = props
	if root["type"] == nil {
		root["type"] = "object"
	}
	out, err := json.Marshal(root)
	if err != nil {
		return e.Spec.Schema
	}
	return out
}

/*
splitFields takes the layer's fields argument out of a call's
arguments.
*/
func splitFields(args json.RawMessage) (json.RawMessage, []string) {
	body := map[string]json.RawMessage{}
	if json.Unmarshal(args, &body) != nil {
		return args, nil
	}
	raw, ok := body[fieldsArgument]
	if !ok {
		return args, nil
	}
	var fields []string
	_ = json.Unmarshal(raw, &fields)
	delete(body, fieldsArgument)
	out, err := json.Marshal(body)
	if err != nil {
		return args, fields
	}
	return out, fields
}

/*
groupNames lists the groups, for a refusal.
*/
func (s *Set) groupNames() string {
	names := make([]string, 0, len(s.groups))
	for _, g := range s.groups {
		names = append(names, g.Name)
	}
	return strings.Join(names, ", ")
}

/*
decodeArgs reads a layer tool's arguments.
*/
func decodeArgs(args json.RawMessage, v any) error {
	if err := json.Unmarshal(orEmpty(args), v); err != nil {
		return fmt.Errorf("%w: arguments must be a JSON object", ErrArguments)
	}
	return nil
}

/*
orEmpty is the arguments, or an empty object when there are none.
*/
func orEmpty(args json.RawMessage) json.RawMessage {
	if len(args) == 0 || string(args) == "null" {
		return json.RawMessage("{}")
	}
	return args
}

/*
mustJSON writes a value as JSON, empty when it cannot.
*/
func mustJSON(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}

/*
firstSentence is a description's first sentence.
*/
func firstSentence(text string) string {
	if i := strings.Index(text, ". "); i > 0 {
		return text[:i+1]
	}
	return text
}
