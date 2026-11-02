/*
|--------------------------------------------------------------------------
| Path Patterns
|--------------------------------------------------------------------------
|
| Lists of paths that get special treatment, such as the paths that keep
| answering during maintenance or that a route lint skips, are written
| the same way everywhere: a path matches itself, and a pattern ending in
| "*" matches every path starting with the rest, so "/webhooks/*" covers
| each inbound webhook.
|
*/

package http

import "strings"

/*
MatchesPath reports whether the path matches any of the patterns.
*/
func MatchesPath(path string, patterns ...string) bool {
	for _, pattern := range patterns {
		if base, ok := strings.CutSuffix(pattern, "*"); ok && strings.HasPrefix(path, base) || path == pattern {
			return true
		}
	}

	return false
}
