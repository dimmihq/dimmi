/*
|--------------------------------------------------------------------------
| List Filters
|--------------------------------------------------------------------------
|
| Liquid's filters for lists: split and join, sort, map and where, slice
| and sum, and the rest. They read anything as a list the way a for loop
| does, so a map gives its pairs and a single value a list of itself.
|
*/

package liquid

import (
	"sort"
	"strings"
)

/*
listFilters are the filters for lists.
*/
func listFilters() map[string]Filter {
	return map[string]Filter{
		"size":         func(c *Call) (any, error) { return size(c.Input), nil },
		"split":        splitFilter,
		"join":         joinFilter,
		"first":        func(c *Call) (any, error) { return property(c.Input, "first"), nil },
		"last":         func(c *Call) (any, error) { return property(c.Input, "last"), nil },
		"reverse":      reverseFilter,
		"sort":         sortFilter(func(v any) any { return v }),
		"sort_natural": sortFilter(func(v any) any { return strings.ToLower(toString(v)) }),
		"uniq":         uniqFilter,
		"compact":      compactFilter,
		"concat":       concatFilter,
		"map":          mapFilter,
		"where":        whereFilter,
		"slice":        sliceFilter,
		"sum":          sumFilter,
	}
}

func splitFilter(c *Call) (any, error) {
	s, sep := toString(c.Input), toString(c.Arg(0))
	if s == "" {
		return []any{}, nil
	}
	var parts []string
	if sep == " " {
		parts = strings.Fields(s)
	} else {
		parts = strings.Split(s, sep)
	}
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

func joinFilter(c *Call) (any, error) {
	sep := " "
	if c.Arg(0) != nil {
		sep = toString(c.Arg(0))
	}
	items := list(c.Input)
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = toString(item)
	}
	return checkText(strings.Join(parts, sep))
}

func reverseFilter(c *Call) (any, error) {
	items := list(c.Input)
	out := make([]any, len(items))
	for i, item := range items {
		out[len(items)-1-i] = item
	}
	return out, nil
}

/*
sortFilter sorts a list, or its items by a key, comparing each value as
normalize leaves it.
*/
func sortFilter(normalize func(any) any) Filter {
	return func(c *Call) (any, error) {
		items := append([]any(nil), list(c.Input)...)
		key := toString(c.Arg(0))
		value := func(v any) any {
			if key != "" {
				v = property(v, key)
			}
			return normalize(v)
		}
		sort.SliceStable(items, func(i, j int) bool {
			a, b := value(items[i]), value(items[j])
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			order, _ := compare(a, b)
			return order < 0
		})
		return items, nil
	}
}

func uniqFilter(c *Call) (any, error) {
	var out []any
	for _, item := range list(c.Input) {
		seen := false
		for _, kept := range out {
			if equal(kept, item) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, item)
		}
	}
	return out, nil
}

func compactFilter(c *Call) (any, error) {
	out := []any{}
	for _, item := range list(c.Input) {
		if item != nil {
			out = append(out, item)
		}
	}
	return out, nil
}

func concatFilter(c *Call) (any, error) {
	return append(append([]any{}, list(c.Input)...), list(c.Arg(0))...), nil
}

func mapFilter(c *Call) (any, error) {
	key := toString(c.Arg(0))
	items := list(c.Input)
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = property(item, key)
	}
	return out, nil
}

func whereFilter(c *Call) (any, error) {
	key := toString(c.Arg(0))
	out := []any{}
	for _, item := range list(c.Input) {
		v := property(item, key)
		if len(c.Args) > 1 {
			if equal(v, c.Args[1]) {
				out = append(out, item)
			}
		} else if truthy(v) {
			out = append(out, item)
		}
	}
	return out, nil
}

func sliceFilter(c *Call) (any, error) {
	start := toInt(c.Arg(0))
	length := 1
	if c.Arg(1) != nil {
		length = toInt(c.Arg(1))
	}
	cut := func(n int) (int, int) {
		if start < 0 {
			start += n
		}
		from := min(max(0, start), n)
		return from, min(n, from+max(0, length))
	}
	if items, ok := c.Input.([]any); ok {
		from, to := cut(len(items))
		return items[from:to], nil
	}
	runes := []rune(toString(c.Input))
	from, to := cut(len(runes))
	return string(runes[from:to]), nil
}

func sumFilter(c *Call) (any, error) {
	total, whole := 0.0, true
	key := toString(c.Arg(0))
	for _, item := range list(c.Input) {
		if key != "" {
			item = property(item, key)
		}
		n, isWhole, _ := number(item)
		total += n
		whole = whole && isWhole
	}
	return numeral{total, whole}.result(), nil
}
