/*
|--------------------------------------------------------------------------
| Field Values
|--------------------------------------------------------------------------
|
| Ticket fields and conversation attributes hold the same kinds of
| value, checked the same way whoever writes them. Fit makes a value
| the type's shape or says why it does not fit: text within its length,
| whole numbers for a number, a calendar day as YYYY-MM-DD, a moment as
| RFC 3339 in UTC, one or several of the options, and files, teammates,
| contacts and companies as their public ids. Nothing else is coerced.
|
*/

package fieldvalues

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
The data types a value may have.
*/
const (
	TypeShortText      = "short_text"
	TypeLongText       = "long_text"
	TypeNumber         = "number"
	TypeDecimal        = "decimal"
	TypeCurrency       = "currency"
	TypeBoolean        = "boolean"
	TypeDate           = "date"
	TypeDateTime       = "datetime"
	TypeSingleChoice   = "single_choice"
	TypeMultipleChoice = "multiple_choice"
	TypeFile           = "file"
	TypeMember         = "member"
	TypeContact        = "contact"
	TypeCompany        = "company"
)

/*
The problems a value may have besides those of attributes.
*/
const (
	ProblemTooLong  = "too_long"
	ProblemNotWhole = "not_whole"
)

/*
Text limits of the text types, and how many files one value holds.
*/
const (
	MaxShortText = 255
	MaxLongText  = 10000
	MaxFiles     = 10
)

/*
publicIDShape is what a public id looks like: a prefix, an underscore
and 26 Crockford base32 characters.
*/
var publicIDShape = regexp.MustCompile(`^[a-z]{2,8}_[0-7][0-9a-hjkmnp-tv-z]{25}$`)

/*
Fit makes a value the type's shape, or says why it does not fit.
*/
func Fit(dataType string, options []string, value any) (any, string) {
	switch dataType {
	case TypeShortText:
		return Text(value, MaxShortText)
	case TypeLongText:
		return Text(value, MaxLongText)
	case TypeNumber, TypeDecimal, TypeCurrency:
		return numeric(dataType, value)
	case TypeBoolean:
		return attributes.Coerce(attributes.TypeBoolean, nil, value)
	case TypeDate:
		return day(value)
	case TypeDateTime:
		return moment(value)
	case TypeSingleChoice:
		return attributes.Coerce(attributes.TypeList, options, value)
	case TypeMultipleChoice:
		return choices(options, value)
	case TypeFile:
		return publicIDs(models.PrefixAttachment, value, MaxFiles)
	case TypeMember:
		return publicID(models.PrefixMember, value)
	case TypeContact:
		return publicID(models.PrefixContact, value)
	case TypeCompany:
		return publicID(models.PrefixCompany, value)
	}

	return nil, attributes.ProblemTypeMismatch
}

/*
Text fits text of at most limit characters.
*/
func Text(value any, limit int) (any, string) {
	s, ok := value.(string)
	if !ok {
		return nil, attributes.ProblemTypeMismatch
	}

	if len([]rune(s)) > limit {
		return nil, ProblemTooLong
	}

	return s, ""
}

/*
Empty reports whether a value counts as missing.
*/
func Empty(v any) bool {
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
numeric fits a number; a number holds whole numbers and a currency
amount is rounded to cents.
*/
func numeric(dataType string, value any) (any, string) {
	n, problem := attributes.Coerce(attributes.TypeNumber, nil, value)
	if problem != "" {
		return nil, problem
	}

	f, _ := n.(float64)
	if dataType == TypeNumber && f != math.Trunc(f) {
		return nil, ProblemNotWhole
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
moment fits a date and time, kept as RFC 3339 in UTC to the second.
*/
func moment(value any) (any, string) {
	s, ok := value.(string)
	if !ok {
		return nil, attributes.ProblemTypeMismatch
	}

	for _, layout := range []string{time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.UTC().Truncate(time.Second).Format(time.RFC3339), ""
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
