/*
|--------------------------------------------------------------------------
| Page Address Patterns
|--------------------------------------------------------------------------
|
| Rules about pages use one small glob language that admins can read at
| a glance: a star matches any characters within one path segment, a
| double star matches anything, slashes included. Matching ignores case
| and a trailing slash, because "/Pricing/" and "/pricing" are the same
| page to everyone but a web server. There is no regular expression:
| patterns stay explainable and cannot be made to run for ever.
|
*/

package urlmatch

import "strings"

/*
Match reports whether the value matches the pattern. The whole value
must match, so "/docs/*" matches "/docs/setup" but not "/docs".
*/
func Match(pattern, value string) bool {
	return matchFrom(normalize(pattern), normalize(value))
}

/*
normalize lowers case and drops one trailing slash, keeping a lone "/".
*/
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 1 && strings.HasSuffix(s, "/") {
		return s[:len(s)-1]
	}

	return s
}

/*
matchFrom walks pattern and value together. A star tries every split
point of the value from the shortest; a single star refuses to cross a
slash. Patterns are short, so the backtracking stays cheap.
*/
func matchFrom(p, v string) bool {
	for p != "" {
		if strings.HasPrefix(p, "**") {
			return matchAnyDepth(strings.TrimLeft(p, "*"), v)
		}

		if p[0] == '*' {
			return matchSegment(p[1:], v)
		}

		if v == "" || p[0] != v[0] {
			return false
		}

		p, v = p[1:], v[1:]
	}

	return v == ""
}

/*
matchAnyDepth matches the rest of the pattern after a double star at
every point of the value.
*/
func matchAnyDepth(rest, v string) bool {
	for i := 0; i <= len(v); i++ {
		if matchFrom(rest, v[i:]) {
			return true
		}
	}

	return false
}

/*
matchSegment matches the rest of the pattern after a single star at
every point up to the next slash.
*/
func matchSegment(rest, v string) bool {
	for i := 0; i <= len(v); i++ {
		if matchFrom(rest, v[i:]) {
			return true
		}

		if i < len(v) && v[i] == '/' {
			return false
		}
	}

	return false
}
