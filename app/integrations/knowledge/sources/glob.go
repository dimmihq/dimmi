/*
|--------------------------------------------------------------------------
| Path Patterns
|--------------------------------------------------------------------------
|
| Members choose which documents of a source count with include and
| exclude patterns over each document's path: a single star matches
| within one segment, a double star across segments, so "/docs/**"
| takes a whole section. A document is kept when it matches an include
| pattern, or there are none, and no exclude pattern.
|
*/

package sources

import (
	"path"
	"strings"
)

/*
Match reports whether the pattern matches the whole path. Leading
slashes are ignored on both, so "docs/**" and "/docs/**" agree.
*/
func Match(pattern, value string) bool {
	return matchSegments(split(pattern), split(value))
}

/*
Allowed reports whether a path passes the include and exclude patterns.
*/
func Allowed(include, exclude []string, value string) bool {
	for _, p := range exclude {
		if Match(p, value) {
			return false
		}
	}

	if len(include) == 0 {
		return true
	}

	for _, p := range include {
		if Match(p, value) {
			return true
		}
	}

	return false
}

func split(s string) []string {
	s = strings.Trim(s, "/")
	if s == "" {
		return nil
	}

	return strings.Split(s, "/")
}

/*
matchSegments matches segment by segment, letting a double star stand
for any number of segments, none included.
*/
func matchSegments(pattern, value []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			return matchAnyDepth(pattern[1:], value)
		}

		if len(value) == 0 || !matchSegment(pattern[0], value[0]) {
			return false
		}

		pattern, value = pattern[1:], value[1:]
	}

	return len(value) == 0
}

/*
matchAnyDepth matches the rest of a pattern after a double star at any
depth of the value.
*/
func matchAnyDepth(rest, value []string) bool {
	for i := 0; i <= len(value); i++ {
		if matchSegments(rest, value[i:]) {
			return true
		}
	}

	return false
}

/*
matchSegment matches one segment, a malformed pattern matching nothing.
*/
func matchSegment(pattern, value string) bool {
	ok, err := path.Match(pattern, value)

	return err == nil && ok
}
