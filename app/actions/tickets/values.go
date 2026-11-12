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
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
The data types a field may hold.
*/
const (
	TypeShortText      = "short_text"
	TypeLongText       = "long_text"
	TypeNumber         = "number"
	TypeDecimal        = "decimal"
	TypeCurrency       = "currency"
	TypeBoolean        = "boolean"
	TypeDate           = "date"
	TypeSingleChoice   = "single_choice"
	TypeMultipleChoice = "multiple_choice"
	TypeFile           = "file"
	TypeMember         = "member"
	TypeCompany        = "company"
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
publicIDShape is what a public id looks like: a prefix, an underscore
and 26 Crockford base32 characters.
*/
var publicIDShape = regexp.MustCompile(`^[a-z]{2,8}_[0-7][0-9a-hjkmnp-tv-z]{25}$`)

/*
Text limits of the text types.
*/
const (
	maxShortText = 255
	maxLongText  = 10000
	maxFiles     = 10
)

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
func empty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	}

	return false
}

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
	switch f.DataType {
	case TypeShortText, TypeLongText:
		return text(f.DataType, value)
	case TypeNumber, TypeDecimal, TypeCurrency:
		return numeric(f.DataType, value)
	case TypeBoolean:
		return attributes.Coerce(attributes.TypeBoolean, nil, value)
	case TypeDate:
		return day(value)
	case TypeSingleChoice:
		return attributes.Coerce(attributes.TypeList, f.Options, value)
	case TypeMultipleChoice:
		return choices(f.Options, value)
	case TypeFile:
		return publicIDs(models.PrefixAttachment, value, maxFiles)
	case TypeMember:
		return publicID(models.PrefixMember, value)
	case TypeCompany:
		return publicID(models.PrefixCompany, value)
	}

	return nil, attributes.ProblemTypeMismatch
}

/*
text fits text within the type's length.
*/
func text(dataType string, value any) (any, string) {
	s, ok := value.(string)
	if !ok {
		return nil, attributes.ProblemTypeMismatch
	}

	limit := maxShortText
	if dataType == TypeLongText {
		limit = maxLongText
	}

	if len([]rune(s)) > limit {
		return nil, "too_long"
	}

	return s, ""
}

/*
numeric fits a number; a number field holds whole numbers.
*/
func numeric(dataType string, value any) (any, string) {
	n, problem := attributes.Coerce(attributes.TypeNumber, nil, value)
	if problem != "" {
		return nil, problem
	}

	f, _ := n.(float64)
	if dataType == TypeNumber && f != math.Trunc(f) {
		return nil, "not_whole"
	}

	if dataType == TypeCurrency {
		return math.Round(f*100) / 100, ""
	}

	return f, ""
}

/*
day fits a calendar date, kept as YYYY-MM-DD.
*/
func day(value any) (any, string) {
	s, ok := value.(string)
	if !ok {
		return nil, attributes.ProblemTypeMismatch
	}

	for _, layout := range []string{time.DateOnly, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.Format(time.DateOnly), ""
		}
	}

	return nil, attributes.ProblemTypeMismatch
}

/*
choices fits a list of options, each at most once.
*/
func choices(options []string, value any) (any, string) {
	list, ok := value.([]any)
	if !ok {
		return nil, attributes.ProblemTypeMismatch
	}

	out := []any{}
	for _, x := range list {
		s, ok := x.(string)
		if !ok || !slices.Contains(options, s) {
			return nil, attributes.ProblemInvalidOption
		}

		if !slices.Contains(out, any(s)) {
			out = append(out, s)
		}
	}

	return out, ""
}

/*
publicID fits one public id of the prefix.
*/
func publicID(prefix models.Prefix, value any) (any, string) {
	s, ok := value.(string)
	if !ok || !publicIDShape.MatchString(s) || !strings.HasPrefix(s, string(prefix)+"_") {
		return nil, attributes.ProblemTypeMismatch
	}

	return s, ""
}

/*
publicIDs fits a list of public ids of the prefix.
*/
func publicIDs(prefix models.Prefix, value any, limit int) (any, string) {
	list, ok := value.([]any)
	if !ok || len(list) > limit {
		return nil, attributes.ProblemTypeMismatch
	}

	out := []any{}
	for _, x := range list {
		id, problem := publicID(prefix, x)
		if problem != "" {
			return nil, problem
		}

		out = append(out, id)
	}

	return out, ""
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
