/*
|--------------------------------------------------------------------------
| Origin Patterns
|--------------------------------------------------------------------------
|
| An allowlist entry names one origin, or every subdomain one level below
| a domain with a leading "*." label. Leaving the scheme out accepts both
| http and https. A wildcard reaches exactly one label: it never covers
| the domain itself or deeper subdomains, and never a public suffix such
| as "*.com" or "*.github.io", which would let anyone in. A pattern that
| does not parse matches nothing, so a typo closes access instead of
| opening it.
|
*/

package origin

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

/*
Match reports whether the origin is allowed by the pattern, such as
"https://app.example.com", "example.com:8443" or "https://*.example.com".
The null origin matches no pattern.
*/
func Match(pattern string, o Origin) bool {
	if o.null || o.Scheme == "" {
		return false
	}

	scheme, rest, hasScheme := strings.Cut(strings.TrimSpace(pattern), "://")
	if !hasScheme {
		scheme, rest = o.Scheme, scheme
	}

	base, wildcard := strings.CutPrefix(rest, "*.")

	p, err := Parse(scheme + "://" + base)
	if err != nil || p.Scheme != o.Scheme || p.Port != o.Port {
		return false
	}

	if !wildcard {
		return p.Host == o.Host
	}

	return coversSubdomain(p.Host, o.Host)
}

/*
coversSubdomain reports whether the host is exactly one label below the
base, and the base is a registrable domain rather than an IP address or
a public suffix. A single-label base outside the suffix list, such as
"localhost", counts as a domain so local development can use wildcards.
*/
func coversSubdomain(base, host string) bool {
	if strings.Contains(base, ":") || isIPv4(base) {
		return false
	}

	suffix, icann := publicsuffix.PublicSuffix(base)
	if suffix == base && (icann || strings.Contains(base, ".")) {
		return false
	}

	label, ok := strings.CutSuffix(host, "."+base)

	return ok && label != "" && !strings.Contains(label, ".")
}

/*
isIPv4 reports whether the host is made only of digits and dots, which
Parse accepts only as an IPv4 address.
*/
func isIPv4(host string) bool {
	return strings.Trim(host, "0123456789.") == ""
}
