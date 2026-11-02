/*
|--------------------------------------------------------------------------
| List Queries
|--------------------------------------------------------------------------
|
| Every list endpoint reads the same parameters: filter[field]=a,b,
| sort=-field,other, include=a,b, cursor, limit and q. Each endpoint
| declares which filters, sorts and includes it supports, and anything
| else is refused with a clear error rather than ignored, so a client
| with a typo learns about it instead of silently getting every row.
| Parameters the grammar does not own are left to the endpoint.
|
*/

package pagination

import (
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
)

/*
Default page sizes, used when Allow leaves them at zero.
*/
const (
	DefaultLimit = 25
	MaxLimit     = 100
)

/*
Errors Parse and Decode return. They are Invalid faults whose codes
clients may branch on; the fault names the offending parameter under
"parameter", and ErrInvalidLimit carries the largest page size under
"max".
*/
var (
	ErrInvalidCursor  = fault.New(fault.Invalid, "invalid_cursor", "The cursor is invalid.")
	ErrInvalidFilter  = fault.New(fault.Invalid, "invalid_filter", "This list cannot be filtered by that field.")
	ErrInvalidSort    = fault.New(fault.Invalid, "invalid_sort", "This list cannot be sorted by that field.")
	ErrInvalidInclude = fault.New(fault.Invalid, "invalid_include", "That relation cannot be included.")
	ErrInvalidLimit   = fault.New(fault.Invalid, "invalid_limit", "The limit must be a whole number within range.")
)

/*
Allow is what one endpoint supports.

Filters, Sorts and Includes list the accepted names. DefaultLimit is the
page size when the client sends none, and MaxLimit the largest it may
ask for; zero means DefaultLimit and MaxLimit of this package.
*/
type Allow struct {
	Filters      []string
	Sorts        []string
	Includes     []string
	DefaultLimit int
	MaxLimit     int
}

/*
Query is a parsed list request.

Filters maps each field to the values asked for, split on commas.
Sorts keeps the client's order. Includes names the relations to embed.
Cursor is still encoded: decode it with Cursors.Decode. Limit is always
set, falling back to the endpoint's default. Search is the trimmed q
parameter.
*/
type Query struct {
	Filters  map[string][]string
	Sorts    []Sort
	Includes []string
	Cursor   string
	Limit    int
	Search   string
}

/*
Sort is one sort key, descending when the client prefixed it with "-".
*/
type Sort struct {
	Field string
	Desc  bool
}

/*
Page is one page of results and the cursors around it. A cursor is empty
when there is no page in that direction. Limit is the page size used.
*/
type Page[T any] struct {
	Items      []T
	NextCursor string
	PrevCursor string
	Limit      int
}

/*
Parse reads the list parameters from the query string and checks them
against what the endpoint allows.
*/
func Parse(values url.Values, allow Allow) (Query, error) {
	filters, err := parseFilters(values, allow.Filters)
	if err != nil {
		return Query{}, err
	}

	sorts, err := parseSorts(values.Get("sort"), allow.Sorts)
	if err != nil {
		return Query{}, err
	}

	includes, err := parseList(values.Get("include"), allow.Includes, ErrInvalidInclude)
	if err != nil {
		return Query{}, err
	}

	limit, err := parseLimit(values, allow)
	if err != nil {
		return Query{}, err
	}

	return Query{
		Filters:  filters,
		Sorts:    sorts,
		Includes: includes,
		Cursor:   values.Get("cursor"),
		Limit:    limit,
		Search:   strings.TrimSpace(values.Get("q")),
	}, nil
}

/*
parseFilters reads every filter[field] parameter in name order, so the
same bad request always reports the same field.
*/
func parseFilters(values url.Values, allowed []string) (map[string][]string, error) {
	var filters map[string][]string

	for _, key := range slices.Sorted(maps.Keys(values)) {
		name, ok := filterName(key)
		if !ok {
			continue
		}

		if !slices.Contains(allowed, name) {
			return nil, ErrInvalidFilter.With("parameter", name)
		}

		if filters == nil {
			filters = make(map[string][]string)
		}

		for _, value := range values[key] {
			filters[name] = append(filters[name], split(value)...)
		}
	}

	return filters, nil
}

func filterName(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, "filter[")
	if !ok {
		return "", false
	}

	return strings.CutSuffix(rest, "]")
}

func parseSorts(raw string, allowed []string) ([]Sort, error) {
	var sorts []Sort

	for _, item := range split(raw) {
		field, desc := strings.CutPrefix(item, "-")
		if !slices.Contains(allowed, field) {
			return nil, ErrInvalidSort.With("parameter", item)
		}

		sorts = append(sorts, Sort{Field: field, Desc: desc})
	}

	return sorts, nil
}

/*
parseList reads a comma-separated list and refuses, with the given
fault, the first name not allowed.
*/
func parseList(raw string, allowed []string, refusal *fault.Error) ([]string, error) {
	items := split(raw)
	for _, item := range items {
		if !slices.Contains(allowed, item) {
			return nil, refusal.With("parameter", item)
		}
	}

	return items, nil
}

func parseLimit(values url.Values, allow Allow) (int, error) {
	maxLimit := cmpOr(allow.MaxLimit, MaxLimit)
	if !values.Has("limit") {
		return min(cmpOr(allow.DefaultLimit, DefaultLimit), maxLimit), nil
	}

	limit, err := strconv.Atoi(values.Get("limit"))
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, ErrInvalidLimit.With("parameter", "limit").With("max", maxLimit)
	}

	return limit, nil
}

/*
split cuts a comma-separated value into its trimmed, non-empty parts.
*/
func split(raw string) []string {
	var parts []string

	for part := range strings.SplitSeq(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

func cmpOr(value, fallback int) int {
	if value > 0 {
		return value
	}

	return fallback
}
