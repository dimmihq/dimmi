/*
|--------------------------------------------------------------------------
| Browser Origins
|--------------------------------------------------------------------------
|
| Allowlists of origins decide which sites may call the application from a
| browser, so two spellings of one origin must compare equal and nothing
| else may. Every origin is reduced to its serialized form: lowercase
| scheme, ASCII host with international names in punycode, and a port only
| when it is not the scheme's default. The opaque "null" origin sent by
| sandboxed frames and file pages parses, but never matches an allowlist.
|
*/

package origin

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

/*
Errors Parse returns.

ErrInvalid means the text is not an origin at all: it does not parse, has
no host, carries a bad host name or a bad port. ErrUnsupportedScheme
means a scheme other than http or https. ErrNotOrigin means a URL that
carries more than an origin: a path, a query, a fragment or user info.
*/
var (
	ErrInvalid           = errors.New("origin: invalid origin")
	ErrUnsupportedScheme = errors.New("origin: scheme must be http or https")
	ErrNotOrigin         = errors.New("origin: url has more than scheme, host and port")
)

/*
Origin is a normalized browser origin.

Scheme is http or https. Host is the lowercase ASCII host name, with
international names in punycode, or an IP address without brackets.
Port is empty when it is the scheme's default. The opaque null origin
has none of them and reports IsNull.
*/
type Origin struct {
	Scheme string
	Host   string
	Port   string
	null   bool
}

/*
Parse reads an origin as browsers send it in the Origin header, or as a
person writes it in configuration: case, a trailing slash, the default
port and international host names are normalized away. The literal
"null" parses to the opaque null origin.
*/
func Parse(s string) (Origin, error) {
	s = strings.TrimSpace(s)
	if s == "null" {
		return Origin{null: true}, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return Origin{}, fmt.Errorf("parse origin %q: %w", s, ErrInvalid)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return Origin{}, fmt.Errorf("parse origin %q: %w", s, ErrUnsupportedScheme)
	}

	if !bare(u) {
		return Origin{}, fmt.Errorf("parse origin %q: %w", s, ErrNotOrigin)
	}

	host, okHost := normalizeHost(u.Hostname())
	port, okPort := normalizePort(u.Scheme, u.Port())
	if !okHost || !okPort {
		return Origin{}, fmt.Errorf("parse origin %q: %w", s, ErrInvalid)
	}

	return Origin{Scheme: u.Scheme, Host: host, Port: port}, nil
}

/*
IsNull reports whether this is the opaque null origin.
*/
func (o Origin) IsNull() bool {
	return o.null
}

/*
String returns the serialized origin, such as "https://example.com" or
"http://[::1]:8080", or "null" for the opaque origin.
*/
func (o Origin) String() string {
	if o.null {
		return "null"
	}

	host := o.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	if o.Port == "" {
		return o.Scheme + "://" + host
	}

	return o.Scheme + "://" + host + ":" + o.Port
}

/*
bare reports whether the URL carries nothing beyond scheme, host and
port. A lone trailing slash is allowed, as people often type one.
*/
func bare(u *url.URL) bool {
	return u.Opaque == "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery &&
		u.Fragment == "" && (u.Path == "" || u.Path == "/")
}

/*
normalizeHost returns the canonical form of an IP address, or the
lowercase punycode form of a host name, and reports whether the host is
valid.
*/
func normalizeHost(host string) (string, bool) {
	if host == "" {
		return "", false
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), true
	}

	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", false
	}

	return ascii, true
}

/*
normalizePort drops the scheme's default port and reports whether the
port is a number a server can listen on.
*/
func normalizePort(scheme, port string) (string, bool) {
	if port == "" {
		return "", true
	}

	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", false
	}

	if (scheme == "http" && n == 80) || (scheme == "https" && n == 443) {
		return "", true
	}

	return strconv.Itoa(n), true
}
