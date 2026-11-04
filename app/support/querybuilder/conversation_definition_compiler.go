/*
|--------------------------------------------------------------------------
| View Definition Compiler
|--------------------------------------------------------------------------
|
| Turns a checked definition into one predicate for the list's query.
| Placeholders are numbered only once the whole predicate is assembled,
| so it slots in after whatever the list already bound. A definition may
| hold one search, and only where every join is and: a search inside an
| or would make every other condition meaningless.
|
*/

package querybuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Reference is a row a condition names: its kind by public id prefix, the
id, and where the condition sits, so a view naming a deleted row can say
which condition no longer matches anything.
*/
type Reference struct {
	Prefix string
	ID     ids.UUID
	Path   string
}

/*
Decoder turns a public id of the prefix into the row id.
*/
type Decoder interface {
	/*
		Decode returns the row the public id names.
	*/
	Decode(prefix, public string) (ids.UUID, error)
}

/*
Compilation is what compiling a definition gives: the predicate, the
search it carries when a condition matches text, and the rows it names.
*/
type Compilation struct {
	Predicate  Fragment
	Search     *string
	References []Reference
}

/*
Compiler turns definitions into predicates for one caller. MemberID
stands for me, Now anchors relative times, Decode reads public ids and
Search turns search text into a tsquery, refusing what it cannot parse.
*/
type Compiler struct {
	MemberID ids.UUID
	Now      time.Time
	Decode   Decoder
	Search   func(text string) (string, error)
}

/*
Compile checks the definition and returns its predicate.
*/
func (c Compiler) Compile(d Definition) (Compilation, error) {
	if err := d.shape(); err != nil {
		return Compilation{}, err
	}

	var out Compilation

	groups := make([]compiled, len(d.Groups))

	for i, g := range d.Groups {
		parts := make([]compiled, len(g.Conditions))

		for j, cond := range g.Conditions {
			path := fmt.Sprintf("groups.%d.conditions.%d", i, j)

			part, err := c.condition(path, cond, &out, []string{d.Op, g.Op})
			if err != nil {
				return Compilation{}, err
			}

			parts[j] = part
		}

		groups[i] = join(g.Op, parts)
	}

	whole := join(d.Op, groups)
	out.Predicate = whole.fragment()

	return out, nil
}

/*
errUnsupported refuses an operator the field does not have.
*/
var errUnsupported = errors.New("operator not supported for this field")

/*
condition compiles one condition, collecting what it names into out.
The joins are those of its group and the definition; text search is
allowed only where all of them are and.
*/
func (c Compiler) condition(path string, cond Condition, out *Compilation, joins []string) (compiled, error) {
	field, ok := fieldFor(cond.Field)
	if !ok {
		return compiled{}, &DefinitionError{Path: path + ".field", Reason: "unknown field"}
	}

	if cond.Field == "text" {
		return c.text(path, cond, out, joins)
	}

	part, refs, err := field.compile(c, cond)
	if errors.Is(err, errUnsupported) {
		return compiled{}, &DefinitionError{Path: path + ".op", Reason: err.Error()}
	}

	if err != nil {
		return compiled{}, &DefinitionError{Path: path + ".value", Reason: err.Error()}
	}

	for _, r := range refs {
		r.Path = path
		out.References = append(out.References, r)
	}

	return part, nil
}

/*
text compiles the one search a definition may hold.
*/
func (c Compiler) text(path string, cond Condition, out *Compilation, joins []string) (compiled, error) {
	if cond.Op != "matches" {
		return compiled{}, &DefinitionError{Path: path + ".op", Reason: errUnsupported.Error()}
	}

	plain := !slices.ContainsFunc(joins, func(op string) bool { return op != opAnd })
	if !plain || out.Search != nil {
		return compiled{}, &DefinitionError{Path: path, Reason: "one text condition, joined only by and"}
	}

	var typed string
	if err := json.Unmarshal(cond.Value, &typed); err != nil || strings.TrimSpace(typed) == "" {
		return compiled{}, &DefinitionError{Path: path + ".value", Reason: "search text required"}
	}

	query, err := c.Search(typed)
	if err != nil {
		return compiled{}, &DefinitionError{Path: path + ".value", Reason: err.Error()}
	}

	out.Search = &query

	return compiled{sql: "TRUE"}, nil
}
