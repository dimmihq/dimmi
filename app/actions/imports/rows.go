/*
|--------------------------------------------------------------------------
| Import Rows
|--------------------------------------------------------------------------
|
| Each row becomes the fields of one customer or company. An empty cell
| sends nothing, so an update never clears what is already there. A
| custom attribute is read by its type: numbers, yes or no, dates and
| the options of a list; a key not defined yet is kept as text. A row
| that cannot be read says why in words a teammate can act on.
|
*/

package imports

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
maxTags is how many tags one row may add.
*/
const maxTags = 20

/*
record is what one row says: a customer with the company they belong
to, or a company, and the tags to add.
*/
type record struct {
	line    int
	contact contacts.Fields
	company *companies.Fields
	self    companies.Fields
	tags    []string
}

/*
builder reads rows by a plan, knowing the kind's attribute definitions
by key.
*/
type builder struct {
	kind  string
	plan  Plan
	defs  map[string]models.AttributeDefinition
	names []string
}

/*
newBuilder returns a builder for the kind, plan and definitions.
*/
func newBuilder(kind string, plan Plan, defs []models.AttributeDefinition, columns []string) builder {
	b := builder{kind: kind, plan: plan, defs: map[string]models.AttributeDefinition{}, names: columns}
	for _, d := range defs {
		b.defs[d.Key] = d
	}

	return b
}

/*
build reads the row, or says why it cannot.
*/
func (b builder) build(l line) (record, string) {
	r := record{line: l.number}
	custom := map[string]any{}

	for _, m := range b.plan.Mapping {
		value := l.cell(m.Column)
		if value == "" {
			continue
		}

		if problem := b.set(&r, custom, m.Field, value); problem != "" {
			return r, fmt.Sprintf("%s: %s", b.names[m.Column], problem)
		}
	}

	if len(custom) > 0 {
		r.contact.CustomAttributes, r.self.CustomAttributes = custom, custom
	}

	return r, b.keyProblem(r)
}

/*
set puts one cell's value in its field.
*/
func (b builder) set(r *record, custom map[string]any, field, value string) string {
	switch field {
	case FieldEmail:
		if _, err := mail.ParseAddress(value); err != nil || strings.ContainsAny(value, "<> ") {
			return "not an email address"
		}

		r.contact.Email = &value
	case FieldName:
		r.contact.Name, r.self.Name = &value, &value
	case FieldExternalID:
		r.contact.ExternalID, r.self.ExternalID = &value, &value
	case FieldPhone:
		if contacts.NormalizePhone(&value) == nil {
			return "not a phone number"
		}

		r.contact.Phone = &value
	case FieldDomain:
		r.self.Domain = &value
	case FieldCompanyName:
		r.employer().Name = &value
	case FieldCompanyDomain:
		r.employer().Domain = &value
	case FieldTags:
		r.tags = splitTags(value)
		if len(r.tags) > maxTags {
			return fmt.Sprintf("more than %d tags", maxTags)
		}
	default:
		typed, problem := b.attribute(attributeKey(field), value)
		if problem != "" {
			return problem
		}

		custom[attributeKey(field)] = typed
	}

	return ""
}

/*
employer returns the company the customer's row names, starting one.
*/
func (r *record) employer() *companies.Fields {
	if r.company == nil {
		r.company = &companies.Fields{}
	}

	return r.company
}

/*
keyProblem says why a row cannot be found or added: a customer needs an
email or external id, a company a domain or external id.
*/
func (b builder) keyProblem(r record) string {
	if b.kind == KindCompanies {
		if r.self.ExternalID == nil && companies.NormalizeDomain(r.self.Domain) == nil {
			return "needs a domain or external id"
		}

		return ""
	}

	if r.contact.ExternalID == nil && r.contact.Email == nil {
		return "needs an email or external id"
	}

	return ""
}

/*
attribute reads a custom attribute's cell by its definition's type.
*/
func (b builder) attribute(key, value string) (any, string) {
	def, ok := b.defs[key]
	if !ok {
		return value, ""
	}

	if def.Status != "active" {
		return nil, "the attribute is archived"
	}

	var typed any = value

	switch def.DataType {
	case attributes.TypeNumber:
		f, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
		if err != nil {
			return nil, "not a number"
		}

		typed = f
	case attributes.TypeBoolean:
		v, ok := yesNo(value)
		if !ok {
			return nil, "not yes or no"
		}

		typed = v
	}

	fitted, problem := attributes.Coerce(def.DataType, attributedefs.Options(def), typed)
	if problem != "" {
		return nil, describe(def, problem)
	}

	return fitted, ""
}

/*
describe words an attribute problem for the report.
*/
func describe(def models.AttributeDefinition, problem string) string {
	switch {
	case problem == attributes.ProblemInvalidOption:
		return "not one of " + strings.Join(attributedefs.Options(def), ", ")
	case def.DataType == attributes.TypeDate:
		return "not a date"
	}

	return "not a " + def.DataType
}

/*
yesNo reads a cell as true or false.
*/
func yesNo(value string) (bool, bool) {
	switch strings.ToLower(value) {
	case "true", "yes", "y", "1":
		return true, true
	case "false", "no", "n", "0":
		return false, true
	}

	return false, false
}

/*
splitTags reads a cell of tags separated by commas or semicolons,
without blanks or repeats.
*/
func splitTags(value string) []string {
	var out []string

	for _, t := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '|' }) {
		t = strings.TrimSpace(t)
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}

	return out
}
