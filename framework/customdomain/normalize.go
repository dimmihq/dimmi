/*
|--------------------------------------------------------------------------
| Hostname Normalization
|--------------------------------------------------------------------------
|
| A customer types a hostname however they like: mixed case, a trailing
| dot, letters outside ASCII. Everything downstream (DNS lookups, claims,
| certificates, the Host header of incoming requests) compares the one
| canonical form, so it is computed once, here, when the hostname enters
| the system.
|
| Only names a certificate can be issued for get through: no addresses,
| ports, wildcards or bare public suffixes, and at least two labels.
|
*/

package customdomain

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

/*
ErrInvalidHostname means the input is not a hostname a customer can own
and serve pages on.
*/
var ErrInvalidHostname = errors.New("customdomain: invalid hostname")

/*
Normalize returns the canonical form of the hostname: trimmed, lower
case, without a trailing dot, and with international labels converted
to their ASCII form, so "Café.Example." becomes "xn--caf-dma.example".
*/
func Normalize(host string) (string, error) {
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if name == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidHostname)
	}

	if strings.ContainsAny(name, ":/*@") {
		return "", fmt.Errorf("%w %q: only a bare hostname is accepted", ErrInvalidHostname, host)
	}

	if _, err := netip.ParseAddr(name); err == nil {
		return "", fmt.Errorf("%w %q: an IP address is not a hostname", ErrInvalidHostname, host)
	}

	ascii, err := profile().ToASCII(name)
	if err != nil {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidHostname, host, err)
	}

	if !strings.Contains(ascii, ".") {
		return "", fmt.Errorf("%w %q: needs at least two labels", ErrInvalidHostname, host)
	}

	if _, err := publicsuffix.EffectiveTLDPlusOne(ascii); err != nil {
		return "", fmt.Errorf("%w %q: a public suffix cannot be owned", ErrInvalidHostname, host)
	}

	return ascii, nil
}

/*
IsApex reports whether the normalized hostname is a registrable domain
itself, such as "example.com", rather than a name under one. An apex
cannot carry a CNAME record, so it is routed by address instead.
*/
func IsApex(host string) bool {
	registrable, err := publicsuffix.EffectiveTLDPlusOne(host)

	return err == nil && registrable == host
}

/*
profile is the IDNA processing used for lookups (UTS 46, without the
transitional mapping), checking label and name lengths and allowing
only letters, digits and hyphens in the result.
*/
func profile() *idna.Profile {
	return idna.New(
		idna.MapForLookup(),
		idna.Transitional(false),
		idna.BidiRule(),
		idna.VerifyDNSLength(true),
		idna.StrictDomainName(true),
	)
}
