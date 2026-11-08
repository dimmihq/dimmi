/*
|--------------------------------------------------------------------------
| Compiling Rules
|--------------------------------------------------------------------------
|
| A rule is checked once, when it is saved or published, against the
| catalog and the surface it guards: every field must exist and be
| readable there, every operator must suit the field's type, and every
| value must have the shape its operator needs. The result is a program
| with values already parsed, cheap enough to run on every event.
|
| Access surfaces guard data or spend, so they refuse fields a browser
| could have claimed. Display surfaces only show, hide or route, so they
| accept claimed values.
|
*/

package audience

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

/*
Gate is what a surface protects.
*/
type Gate string

/*
The gates: display shows, hides or routes; access reveals data or
spends something.
*/
const (
	GateDisplay Gate = "display"
	GateAccess  Gate = "access"
)

/*
Surface is a place rules guard: its key, its gate, where its rules are
evaluated, and which fields it may read on top of those locations.
*/
type Surface struct {
	Key   string
	Gate  Gate
	Where []Location
}

/*
Program is a compiled audience, ready to evaluate.
*/
type Program struct {
	visitors string
	root     *node
}

/*
node is a compiled rule node. Leaves carry the field and parsed operand.
*/
type node struct {
	kind     string
	children []*node
	path     string
	field    Field
	key      string
	op       string
	operand  operand
	within   int
	original Rule
}

/*
Compile checks the audience against the catalog and the surface and
returns its program, or every problem found.
*/
func Compile(a Audience, c *Catalog, s Surface) (*Program, error) {
	if err := checkShape(a); err != nil {
		return nil, err
	}

	p := &Program{visitors: a.Visitors}
	var found []Problem
	comp := compiler{catalog: c, surface: s, found: &found}

	var parts []*node
	if lead := visitorRules(a.Visitors); lead != nil {
		parts = append(parts, compileNode(*lead, "visitors", comp))
	}

	if a.Rules != nil {
		parts = append(parts, compileNode(*a.Rules, "rules", comp))
	}

	switch len(parts) {
	case 0:
		return p, nil
	case 1:
		p.root = parts[0]
	default:
		p.root = &node{kind: "all", path: "", children: parts}
	}

	return p, problems(found...)
}

/*
CompileRule compiles a bare rule tree.
*/
func CompileRule(r *Rule, c *Catalog, s Surface) (*Program, error) {
	return Compile(Audience{Rules: r}, c, s)
}

/*
visitorRules turns a visitor group into plain conditions on the contact,
so it evaluates like any other condition.
*/
func visitorRules(visitors string) *Rule {
	leaf := func(field, op, value string) Rule {
		r := Rule{Field: field, Op: op}
		if value != "" {
			r.Value = json.RawMessage(strconv.Quote(value))
		}

		return r
	}

	var lead []Rule

	switch visitors {
	case VisitorsIdentified:
		lead = []Rule{leaf("contact.kind", OpEq, "user")}
	case VisitorsLeads:
		lead = []Rule{leaf("contact.kind", OpEq, "lead"), leaf("contact.email", OpExists, "")}
	case VisitorsAnonymous:
		lead = []Rule{leaf("contact.kind", OpEq, "lead"), leaf("contact.email", OpNotExists, "")}
	case VisitorsAnonymousAndLeads:
		lead = []Rule{leaf("contact.kind", OpEq, "lead")}
	}

	if len(lead) == 0 {
		return nil
	}

	return &Rule{All: lead}
}

/*
compiler carries what compiling a tree needs.
*/
type compiler struct {
	catalog *Catalog
	surface Surface
	found   *[]Problem
}

/*
compileNode compiles one node and its children.
*/
func compileNode(r Rule, path string, c compiler) *node {
	switch {
	case r.Not != nil:
		return &node{kind: "not", path: path, children: []*node{compileNode(*r.Not, path+".not", c)}}
	case r.All != nil:
		return group("all", r.All, path, c)
	case r.Any != nil:
		return group("any", r.Any, path, c)
	}

	return compileLeaf(r, path, c)
}

/*
group compiles an all or any node.
*/
func group(kind string, children []Rule, path string, c compiler) *node {
	n := &node{kind: kind, path: path}
	for i, child := range children {
		n.children = append(n.children, compileNode(child, fmt.Sprintf("%s.%s[%d]", path, kind, i), c))
	}

	return n
}

/*
compileLeaf resolves the field, checks the operator and the surface,
and parses the value.
*/
func compileLeaf(r Rule, path string, c compiler) *node {
	n := &node{kind: "leaf", path: path, key: r.Field, op: r.Op, within: r.WithinDays, original: r}
	fail := func(code, message string) *node {
		*c.found = append(*c.found, Problem{Code: code, Path: path, Field: r.Field, Message: message})
		return n
	}

	f, ok := c.catalog.Lookup(r.Field)
	if !ok {
		return fail(CodeUnknownAttribute, "No field has this name.")
	}

	n.field = f
	if !slices.Contains(allowedOps(f), r.Op) {
		return fail(CodeOpInvalidForType, fmt.Sprintf("A %s field cannot use %q.", f.Type, r.Op))
	}

	if !slices.Contains(c.surface.Where, f.Where) {
		return fail(CodeFieldUnavailable, "This field is not known where this rule runs.")
	}

	if c.surface.Gate == GateAccess && f.Trust == Claimed {
		return fail(CodeClaimedOnAccessGate, "A visitor could claim this value, so it cannot guard access.")
	}

	operand, err := parseOperand(f, r)
	if err != nil {
		return fail(CodeValueInvalid, sentence(err.Error()))
	}

	n.operand = operand

	return n
}

/*
sentence turns a lowercase error into a sentence for people.
*/
func sentence(text string) string {
	if text == "" {
		return text
	}

	return strings.ToUpper(text[:1]) + text[1:] + "."
}
