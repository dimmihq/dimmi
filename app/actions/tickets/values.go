/*
|--------------------------------------------------------------------------
| Field Values
|--------------------------------------------------------------------------
|
| A ticket's field values are checked against its type's fields the
| same way whoever writes them: unknown keys and archived fields are
| refused, each value must fit the field's data type, and choices must
| be among the field's options. A field shown only when another field
| has a value counts as required only while it shows. Members,
| companies and files are kept as their public ids.
|
*/

package tickets

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/fieldvalues"
)

/*
The data types a field may hold.
*/
const (
	TypeShortText      = fieldvalues.TypeShortText
	TypeLongText       = fieldvalues.TypeLongText
	TypeNumber         = fieldvalues.TypeNumber
	TypeDecimal        = fieldvalues.TypeDecimal
	TypeCurrency       = fieldvalues.TypeCurrency
	TypeBoolean        = fieldvalues.TypeBoolean
	TypeDate           = fieldvalues.TypeDate
	TypeSingleChoice   = fieldvalues.TypeSingleChoice
	TypeMultipleChoice = fieldvalues.TypeMultipleChoice
	TypeFile           = fieldvalues.TypeFile
	TypeMember         = fieldvalues.TypeMember
	TypeCompany        = fieldvalues.TypeCompany
)

/*
DataTypes returns every data type in the order builders offer them.
*/
func DataTypes() []string {
	return []string{
		TypeShortText, TypeLongText, TypeNumber, TypeDecimal, TypeCurrency, TypeBoolean, TypeDate, TypeSingleChoice,
		TypeMultipleChoice, TypeFile, TypeMember, TypeCompany,
	}
}

/*
ShowWhen is the condition a field shows under: another field has a
value, one of Equals when it lists any.
*/
type ShowWhen struct {
	Field  string   `json:"field"`
	Equals []string `json:"equals,omitempty"`
}

/*
showWhenOf reads a field's condition, nil when it always shows.
*/
func showWhenOf(f models.TicketField) *ShowWhen {
	if len(f.ShowWhen) == 0 || string(f.ShowWhen) == "null" {
		return nil
	}

	var sw ShowWhen
	if json.Unmarshal(f.ShowWhen, &sw) != nil || sw.Field == "" {
		return nil
	}

	return &sw
}

/*
shows reports whether a field shows given the values: it always does
without a condition, else when the other field has a value matching.
*/
func shows(f models.TicketField, values map[string]any) bool {
	sw := showWhenOf(f)
	if sw == nil {
		return true
	}

	v, ok := values[sw.Field]
	if !ok || empty(v) {
		return false
	}

	if len(sw.Equals) == 0 {
		return true
	}

	for _, s := range textsOf(v) {
		if slices.Contains(sw.Equals, s) {
			return true
		}
	}

	return false
}

/*
textsOf reads a value as the texts it holds, for show conditions.
*/
func textsOf(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}

		return out
	case bool:
		if t {
			return []string{"true"}
		}

		return []string{"false"}
	}

	return nil
}

/*
empty reports whether a value counts as missing.
*/
func empty(v any) bool { return fieldvalues.Empty(v) }

/*
merge applies changes to stored values: a null removes a key. Each
changed value is checked against the type's fields; problems come back
keyed by field.
*/
func merge(fields []models.TicketField, stored, changes map[string]any) (map[string]any, map[string]string) {
	out := map[string]any{}
	maps.Copy(out, stored)

	problems := map[string]string{}

	for key, value := range changes {
		f, ok := fieldByKey(fields, key)

		switch {
		case !ok:
			problems[key] = "unknown_field"
		case f.ArchivedAt != nil:
			problems[key] = "archived_field"
		case value == nil:
			delete(out, key)
		default:
			fitted, problem := fit(f, value)
			if problem != "" {
				problems[key] = problem
				continue
			}

			out[key] = fitted
		}
	}

	return out, problems
}

/*
fieldByKey returns the field with the key.
*/
func fieldByKey(fields []models.TicketField, key string) (models.TicketField, bool) {
	i := slices.IndexFunc(fields, func(f models.TicketField) bool { return f.Key == key })
	if i < 0 {
		return models.TicketField{}, false
	}

	return fields[i], true
}

/*
fit makes a value the field's type, or says why it does not fit.
*/
func fit(f models.TicketField, value any) (any, string) {
	return fieldvalues.Fit(f.DataType, f.Options, value)
}

/*
missingFor returns the keys of shown, live fields with no value that
the check requires: to create, by the team or the customer, or to
resolve.
*/
func missingFor(fields []models.TicketField, values map[string]any, need func(models.TicketField) bool) []string {
	var out []string
	for _, f := range fields {
		if f.ArchivedAt != nil || !need(f) || !shows(f, values) {
			continue
		}

		if v, ok := values[f.Key]; !ok || empty(v) {
			out = append(out, f.Key)
		}
	}

	return out
}

/*
requiredToCreate says what a create by the team, the API or automation
must fill.
*/
func requiredToCreate(f models.TicketField) bool { return f.RequiredCreate }

/*
requiredFromCustomer says what the customer's form must fill.
*/
func requiredFromCustomer(f models.TicketField) bool { return f.RequiredCreateCustomer }

/*
requiredFor returns what the create must fill: the customer's own
fields for their form, the team's for everyone else.
*/
func requiredFor(in CreateInput) func(models.TicketField) bool {
	if in.Customer {
		return requiredFromCustomer
	}

	return requiredToCreate
}

/*
requiredToResolve says what entering a resolved state needs.
*/
func requiredToResolve(f models.TicketField) bool { return f.RequiredResolve }

/*
missingError reports the missing keys as one refusal.
*/
func missingError(keys []string) error {
	return ErrMissingFields.With("missing", keys)
}

/*
invalidError reports the values that do not fit as one refusal.
*/
func invalidError(problems map[string]string) error {
	return ErrFieldInvalid.With("problems", problems)
}
