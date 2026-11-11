/*
|--------------------------------------------------------------------------
| Website Addresses
|--------------------------------------------------------------------------
|
| One page has one address. Scheme and host are lowercased, the default
| port and the fragment dropped, tracking parameters removed and the
| rest sorted, so links written differently still meet. The crawl stays
| on the site it started on: the start host, with or without "www.".
|
*/

package website

import (
	"net/url"
	"strings"
)

/*
Canonical writes the address in its one canonical form.
*/
func Canonical(u *url.URL) string {
	c := *u
	c.Scheme = strings.ToLower(c.Scheme)
	c.Host = strings.ToLower(c.Host)
	c.Fragment, c.RawFragment = "", ""

	if (c.Scheme == "https" && c.Port() == "443") || (c.Scheme == "http" && c.Port() == "80") {
		c.Host = c.Hostname()
	}

	if c.Path == "" {
		c.Path = "/"
	}

	query := c.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "ref" || lower == "fbclid" || lower == "gclid" {
			query.Del(key)
		}
	}

	c.RawQuery = query.Encode()

	return c.String()
}

/*
sameSite reports whether the address is on the start's site.
*/
func sameSite(start, u *url.URL) bool {
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return bareHost(start.Hostname()) == bareHost(u.Hostname())
}

func bareHost(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}
