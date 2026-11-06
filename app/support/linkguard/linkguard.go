/*
|--------------------------------------------------------------------------
| Link Guard
|--------------------------------------------------------------------------
|
| The model writes links; only the ones the workspace allows reach a
| customer. The allowlist is the help center's own hosts plus the
| domains the workspace named; everything else is stripped to its
| bare URL text or dropped.
|
*/

package linkguard

import (
	"net/url"
	"regexp"
	"strings"
)

/*
mdLink matches [text](url).
*/
var mdLink = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)`)

/*
bareURL matches a raw http(s) URL.
*/
var bareURL = regexp.MustCompile(`https?://[^\s)\]]+`)

/*
Filter removes links whose host is not allowed. Allowed hosts come
from the knowledge index's public hosts plus the agent's own
allowlist; a matching suffix counts, so a domain covers its
subdomains.
*/
func Filter(text string, allowed []string) string {
	if len(allowed) == 0 {
		return mdLink.ReplaceAllString(text, "$1")
	}

	out := mdLink.ReplaceAllStringFunc(text, func(m string) string {
		parts := mdLink.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		if hostAllowed(parts[2], allowed) {
			return m
		}
		return parts[1]
	})

	out = bareURL.ReplaceAllStringFunc(out, func(m string) string {
		if hostAllowed(m, allowed) {
			return m
		}
		return ""
	})

	return strings.TrimSpace(out)
}

/*
hostAllowed reports whether a URL's host matches the allowlist.
*/
func hostAllowed(raw string, allowed []string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}
