/*
|--------------------------------------------------------------------------
| Redirect URIs
|--------------------------------------------------------------------------
|
| The redirect URI is where an authorization code travels, so it decides
| who can steal one. A registered URI must be HTTPS, plain HTTP on the
| user's own machine, or an app's own scheme; schemes a browser would run
| or render itself are refused. A request's URI must equal a registered
| one character for character, with one exception: a command-line app
| listening on the loopback interface cannot know its port in advance,
| so loopback URIs match on any port.
|
*/

package oauth

import (
	"net"
	"net/url"
	"slices"
	"strings"
)

/*
blockedSchemes lists the schemes a browser handles itself, which can
never deliver a code to an app.
*/
func blockedSchemes() []string {
	return []string{"javascript", "data", "file", "vbscript", "about", "blob", "ws", "wss"}
}

/*
checkRedirect returns an invalid_redirect_uri Error for a URI a client
may not register.
*/
func checkRedirect(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Fragment != "" || strings.Contains(raw, "#") || u.User != nil ||
		strings.Contains(raw, "*") {
		return refuse(CodeInvalidRedirectURI, "Redirect URIs must be absolute, without fragment or wildcard.")
	}

	switch {
	case slices.Contains(blockedSchemes(), u.Scheme):
		return refuse(CodeInvalidRedirectURI, "Redirect URIs may not use the "+u.Scheme+" scheme.")
	case u.Scheme == "http" && !loopback(u.Hostname()):
		return refuse(CodeInvalidRedirectURI, "Plain http redirect URIs must point at a loopback address.")
	case u.Scheme == "https" && u.Host == "":
		return refuse(CodeInvalidRedirectURI, "https redirect URIs must name a host.")
	}

	return nil
}

/*
matchRedirect returns the URI the answer goes to: the one asked for
when it matches a registered URI, or the only registered URI when none
is asked for.
*/
func matchRedirect(registered []string, asked string) (string, bool) {
	if asked == "" {
		if len(registered) != 1 {
			return "", false
		}

		return registered[0], true
	}

	for _, uri := range registered {
		if uri == asked || loopbackMatch(uri, asked) {
			return asked, true
		}
	}

	return "", false
}

/*
loopbackMatch reports whether both URIs are plain http on the same
loopback host, path and query, whatever their ports.
*/
func loopbackMatch(registered, asked string) bool {
	r, err := url.Parse(registered)
	if err != nil || r.Scheme != "http" || !loopback(r.Hostname()) {
		return false
	}

	a, err := url.Parse(asked)
	if err != nil || a.Fragment != "" || a.User != nil {
		return false
	}

	return a.Scheme == r.Scheme && a.Hostname() == r.Hostname() && a.EscapedPath() == r.EscapedPath() &&
		a.RawQuery == r.RawQuery
}

/*
loopback reports whether the host is localhost or a loopback address.
*/
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
