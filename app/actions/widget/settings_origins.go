/*
|--------------------------------------------------------------------------
| Allowed Sites
|--------------------------------------------------------------------------
|
| The sites a widget accepts requests from, always stored in one normal
| form, so a list compares the way browsers send Origin: lowercase, no
| default port, international names in their ASCII form. A wildcard covers
| exactly one leftmost label and never the bare domain, because a wildcard
| that catches more than its owner meant is how sites get impersonated.
|
| This is a browser's defense against other sites embedding a widget, not
| authentication: any program can send any header.
|
*/

package widget

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/http/origin"
)

/*
maxAllowedOrigins is the most sites one widget lists.
*/
const maxAllowedOrigins = 50

/*
AppOrigin is what install status records for a request with no Origin
header, the mark of a native app.
*/
const AppOrigin = "app"

/*
NormalizeOrigins returns the sites in their normal form without duplicates,
refusing any that is not an origin.
*/
func NormalizeOrigins(raw []string) ([]string, error) {
	if len(raw) > maxAllowedOrigins {
		return nil, failField("allowed_origins", fmt.Sprintf("List at most %d sites.", maxAllowedOrigins))
	}

	out := make([]string, 0, len(raw))

	for i, entry := range raw {
		normal, ok := normalizeOrigin(entry)
		if !ok {
			return nil, failField(fmt.Sprintf("allowed_origins.%d", i), "Origin must be scheme://host[:port], no path.")
		}

		if !slices.Contains(out, normal) {
			out = append(out, normal)
		}
	}

	return out, nil
}

/*
normalizeOrigin returns one site in its normal form. A wildcard is checked
by what it would match, which refuses one covering a public suffix.
*/
func normalizeOrigin(entry string) (string, bool) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(entry), "://")
	if !found {
		return "", false
	}

	base, wildcard := strings.CutPrefix(rest, "*.")
	if strings.Contains(base, "*") {
		return "", false
	}

	parsed, err := origin.Parse(scheme + "://" + base)
	if err != nil || parsed.IsNull() {
		return "", false
	}

	if !wildcard {
		return parsed.String(), true
	}

	pattern := strings.Replace(parsed.String(), "://", "://*.", 1)

	sample, err := origin.Parse(scheme + "://probe." + base)
	if err != nil || !origin.Match(pattern, sample) {
		return "", false
	}

	return pattern, true
}

/*
OriginAllowed decides whether a request's Origin header may use a widget,
given the install's own origin. A blank header is a client without one, such
as a native app; a literal null comes from a sandboxed page and only passes
an empty list.
*/
func OriginAllowed(allowed []string, nativeApps bool, header, install string) bool {
	if len(allowed) == 0 {
		return true
	}

	if header == "" {
		return nativeApps
	}

	parsed, err := origin.Parse(header)
	if err != nil || parsed.IsNull() {
		return false
	}

	if parsed.String() == install {
		return true
	}

	return slices.ContainsFunc(allowed, func(pattern string) bool { return origin.Match(pattern, parsed) })
}
