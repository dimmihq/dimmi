/*
|--------------------------------------------------------------------------
| Audience Rules
|--------------------------------------------------------------------------
|
| One rule language decides who sees, gets or triggers anything: the
| messenger, workflows, help articles, the agent's knowledge. A rule is
| a small JSON tree of "all", "any" and "not" groups over leaves that
| compare one field with one operator. Only this package reads that
| JSON, so every surface agrees on what a rule means.
|
| Rules stay small enough to read and to evaluate on every request:
| four levels deep, fifty leaves and eight kilobytes at most.
|
*/

package audience

import (
	"bytes"
	"encoding/json"
	"fmt"
)

/*
Limits every rule keeps.
*/
const (
	MaxDepth    = 4
	MaxLeaves   = 50
	MaxBytes    = 8 * 1024
	MaxInValues = 50
)

/*
The visitor groups an audience may start from: Everyone, signed-in
users, leads who left an email, anonymous visitors, or both kinds of
lead.
*/
const (
	VisitorsEveryone          = "everyone"
	VisitorsIdentified        = "identified"
	VisitorsLeads             = "leads"
	VisitorsAnonymous         = "anonymous"
	VisitorsAnonymousAndLeads = "anonymous_and_leads"
)

/*
Audience is a stored audience: which visitors it starts from and the
rule they must also meet. A nil Rules means no further rule.
*/
type Audience struct {
	Visitors string `json:"visitors,omitempty"`
	Rules    *Rule  `json:"rules,omitempty"`
}

/*
Rule is one node of a rule tree. A group sets exactly one of All, Any
or Not; a leaf sets Field and Op, and usually Value. WithinDays narrows
event counts to the last days.
*/
type Rule struct {
	All        []Rule          `json:"all,omitempty"`
	Any        []Rule          `json:"any,omitempty"`
	Not        *Rule           `json:"not,omitempty"`
	Field      string          `json:"field,omitempty"`
	Op         string          `json:"op,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	WithinDays int             `json:"within_days,omitempty"`
}

/*
IsLeaf reports whether the node compares a field rather than grouping.
*/
func (r Rule) IsLeaf() bool {
	return r.All == nil && r.Any == nil && r.Not == nil
}

/*
Parse reads a stored audience. Unknown keys and malformed nodes are
problems with the JSON path of the node, so the builder can point at
the leaf to fix.
*/
func Parse(raw json.RawMessage) (Audience, error) {
	var a Audience
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return a, nil
	}

	if len(raw) > MaxBytes {
		return a, problems(Problem{Code: CodeLimitExceeded, Path: "", Message: "The rule is larger than 8 KB."})
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&a); err != nil {
		return a, problems(Problem{Code: CodeSchema, Message: fmt.Sprintf("The rule is not valid JSON: %v.", err)})
	}

	return a, checkShape(a)
}

/*
ParseRule reads a bare rule tree, as a branch condition stores it.
*/
func ParseRule(raw json.RawMessage) (*Rule, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}

	a, err := Parse(fmt.Appendf(nil, `{"rules":%s}`, raw))
	if err != nil {
		return nil, err
	}

	return a.Rules, nil
}

/*
checkShape refuses unknown visitor groups and nodes that are neither a
clean group nor a clean leaf, and enforces depth and leaf limits.
*/
func checkShape(a Audience) error {
	var found []Problem

	switch a.Visitors {
	case "", VisitorsEveryone, VisitorsIdentified, VisitorsLeads, VisitorsAnonymous, VisitorsAnonymousAndLeads:
	default:
		found = append(found, Problem{Code: CodeSchema, Path: "visitors", Message: "Unknown visitor group."})
	}

	if a.Rules != nil {
		leaves := 0
		found = append(found, shapeOf(*a.Rules, "rules", 1, &leaves)...)

		if leaves > MaxLeaves {
			found = append(found, Problem{
				Code: CodeLimitExceeded, Path: "rules", Message: "Use 50 conditions at most.",
			})
		}
	}

	return problems(found...)
}

/*
shapeOf checks one node and its children.
*/
func shapeOf(r Rule, path string, depth int, leaves *int) []Problem {
	if r.IsLeaf() {
		*leaves++

		if r.Field == "" || r.Op == "" {
			return []Problem{{Code: CodeSchema, Path: path, Message: "A condition needs a field and an operator."}}
		}

		return nil
	}

	if r.Field != "" || r.Op != "" || len(r.Value) > 0 || groupCount(r) != 1 {
		return []Problem{{Code: CodeSchema, Path: path, Message: "A group holds exactly one of all, any or not."}}
	}

	if depth > MaxDepth {
		return []Problem{{Code: CodeLimitExceeded, Path: path, Message: "Nest groups four levels deep at most."}}
	}

	if r.Not != nil {
		return shapeOf(*r.Not, path+".not", depth+1, leaves)
	}

	key, children := "all", r.All
	if r.Any != nil {
		key, children = "any", r.Any
	}

	if len(children) == 0 {
		return []Problem{{Code: CodeSchema, Path: path + "." + key, Message: "A group needs at least one condition."}}
	}

	var found []Problem
	for i, child := range children {
		found = append(found, shapeOf(child, fmt.Sprintf("%s.%s[%d]", path, key, i), depth+1, leaves)...)
	}

	return found
}

/*
groupCount counts how many group keys a node sets.
*/
func groupCount(r Rule) int {
	n := 0
	for _, set := range []bool{r.All != nil, r.Any != nil, r.Not != nil} {
		if set {
			n++
		}
	}

	return n
}
