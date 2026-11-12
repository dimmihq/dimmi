/*
|--------------------------------------------------------------------------
| Import Mapping
|--------------------------------------------------------------------------
|
| A mapping says which column fills which field. Customers take an
| email, a name, an external id, a phone, their company's name or domain
| and tags; companies take a name, a domain, an external id and tags.
| Either can fill any custom attribute of its kind, named by its key, and
| a key not defined yet is defined by the import. Every row needs
| something to be found by: an email or external id for a customer, a
| domain or external id for a company. Columns left out of the mapping
| are ignored.
|
*/

package imports

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
The kinds of import.
*/
const (
	KindContacts  = "contacts"
	KindCompanies = "companies"
)

/*
What a row that matches someone already there does: change them, or
leave them alone.
*/
const (
	ModeUpdate = "update"
	ModeSkip   = "skip"
)

/*
The fields a column can fill, besides custom attributes.
*/
const (
	FieldEmail         = "email"
	FieldName          = "name"
	FieldExternalID    = "external_id"
	FieldPhone         = "phone"
	FieldDomain        = "domain"
	FieldCompanyName   = "company_name"
	FieldCompanyDomain = "company_domain"
	FieldTags          = "tags"
	attributePrefix    = "attribute."
)

/*
ErrMapping refuses a mapping that names a column the file lacks, a
field the kind does not take, a field twice, or no field to find rows
by.
*/
var ErrMapping = fault.New(fault.Invalid, "import_mapping_invalid",
	"Map each field once, from a column of the file, and map an email or external id "+
		"(a domain or external id for companies).").OnField("mapping")

/*
ErrMode refuses a mode that is not update or skip.
*/
var ErrMode = fault.New(fault.Invalid, "import_mode_invalid", "Choose update or skip.").OnField("mode")

/*
Mapping is one column and the field it fills.
*/
type Mapping struct {
	Column int    `json:"column"`
	Field  string `json:"field"`
}

/*
Plan is how a file is read: its mapping and what a matching row does.
*/
type Plan struct {
	Mapping []Mapping
	Mode    string
}

/*
fieldsOf returns the fields the kind takes besides custom attributes.
*/
func fieldsOf(kind string) []string {
	if kind == KindCompanies {
		return []string{FieldName, FieldDomain, FieldExternalID, FieldTags}
	}

	return []string{FieldEmail, FieldName, FieldExternalID, FieldPhone, FieldCompanyName, FieldCompanyDomain, FieldTags}
}

/*
check refuses a plan the file and kind cannot run.

Errors: [ErrMapping] or [ErrMode].
*/
func (p Plan) check(kind string, columns int) error {
	if p.Mode != ModeUpdate && p.Mode != ModeSkip {
		return ErrMode
	}

	seen := map[string]bool{}

	for _, m := range p.Mapping {
		known := slices.Contains(fieldsOf(kind), m.Field) ||
			(strings.HasPrefix(m.Field, attributePrefix) && attributes.ValidKey(attributeKey(m.Field)))
		if m.Column < 0 || m.Column >= columns || !known || seen[m.Field] {
			return ErrMapping.With("field", m.Field)
		}

		seen[m.Field] = true
	}

	keys := []string{FieldEmail, FieldExternalID}
	if kind == KindCompanies {
		keys = []string{FieldDomain, FieldExternalID}
	}

	if !seen[keys[0]] && !seen[keys[1]] {
		return ErrMapping
	}

	return nil
}

/*
attributeKey returns the custom attribute a field names.
*/
func attributeKey(field string) string {
	return strings.TrimPrefix(field, attributePrefix)
}

/*
fieldAliases returns the headers that suggest each field, normalized.
*/
func fieldAliases() map[string][]string {
	return map[string][]string{
		FieldEmail:      {"email", "email_address", "e_mail", "mail"},
		FieldName:       {"name", "full_name", "contact_name", "display_name"},
		FieldExternalID: {"external_id", "user_id", "userid", "id", "customer_id", "company_id"},
		FieldPhone:      {"phone", "phone_number", "mobile", "telephone"},
		FieldDomain:     {"domain", "website", "company_domain", "url"},
		FieldCompanyName: {
			"company", "company_name", "organization",
			"organisation", //nolint:misspell // the British spelling teammates type as a header
			"account",
		},
		FieldCompanyDomain: {"company_domain", "domain", "website", "company_website"},
		FieldTags:          {"tags", "tag", "labels", "segments"},
	}
}

/*
suggest maps the columns whose headers name a field or a defined custom
attribute, each field at most once.
*/
func suggest(kind string, columns []string, defs []models.AttributeDefinition) []Mapping {
	out := []Mapping{}
	taken := map[string]bool{}
	aliases := fieldAliases()

	for i, header := range columns {
		h := normalize(header)
		if field := suggestField(kind, h, defs, aliases); field != "" && !taken[field] {
			taken[field] = true
			out = append(out, Mapping{Column: i, Field: field})
		}
	}

	return out
}

/*
suggestField returns the field the normalized header names, by the
aliases of each field, empty for none.
*/
func suggestField(kind, h string, defs []models.AttributeDefinition, aliases map[string][]string) string {
	if kind == KindCompanies && (h == "name" || h == "company_name" || h == "company") {
		return FieldName
	}

	for _, field := range fieldsOf(kind) {
		if slices.Contains(aliases[field], h) {
			return field
		}
	}

	for _, def := range defs {
		if def.Status == "active" && (def.Key == h || normalize(def.Label) == h) {
			return attributePrefix + def.Key
		}
	}

	return ""
}

/*
normalize returns the header in lower case with runs of other
characters as one underscore.
*/
func normalize(header string) string {
	var b strings.Builder

	gap := false

	for _, r := range strings.ToLower(strings.TrimSpace(header)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if gap && b.Len() > 0 {
				b.WriteByte('_')
			}

			b.WriteRune(r)
			gap = false

			continue
		}

		gap = true
	}

	return b.String()
}
