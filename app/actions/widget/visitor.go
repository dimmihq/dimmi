/*
|--------------------------------------------------------------------------
| Visitor Details
|--------------------------------------------------------------------------
|
| What a visitor's browser says about where they are is kept with their
| session and shown to the team, so it is kept small and harmless first:
| page addresses lose query values that look like credentials, the network
| address is cut to a prefix, and page context is bounded in size, keys and
| shape. None of it is trusted for anything but display.
|
*/

package widget

import (
	"encoding/json"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
Limits on what a visitor's browser sends about them.
*/
const (
	maxPageURL      = 2048
	maxPageTitle    = 300
	maxRecentPages  = 10
	maxUserAgent    = 512
	maxContextBytes = 4 << 10
	maxContextKeys  = 30
	maxContextValue = 500
)

/*
Visitor is where a request came from: the site's origin, the browser and the
network address, which is never stored in full.
*/
type Visitor struct {
	Origin    string
	UserAgent string
	IP        string
}

/*
Page is one page a visitor was on.
*/
type Page struct {
	URL      string     `json:"url"`
	Title    string     `json:"title,omitempty"`
	Referrer string     `json:"referrer,omitempty"`
	At       *time.Time `json:"at,omitempty"`
}

/*
secretQueryKey matches query keys whose values must never be stored.
*/
func secretQueryKey() *regexp.Regexp {
	return regexp.MustCompile(`(?i)token|key|secret|password|code|session`)
}

/*
CleanPageURL returns the address without query values that look like
credentials, or "" for text that is no web address.
*/
func CleanPageURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}

	u.User, u.Fragment = nil, ""

	query := u.Query()
	secret := secretQueryKey()

	for key := range query {
		if secret.MatchString(key) {
			query.Del(key)
		}
	}

	u.RawQuery = query.Encode()

	cleaned := u.String()
	if len(cleaned) > maxPageURL {
		return ""
	}

	return cleaned
}

/*
cleanPage returns the page fit to store, and false when it has no usable
address.
*/
func cleanPage(p Page, at time.Time) (Page, bool) {
	p.URL = CleanPageURL(p.URL)
	if p.URL == "" {
		return Page{}, false
	}

	p.Title = truncate(str.Clean(p.Title), maxPageTitle)
	p.Referrer = CleanPageURL(p.Referrer)
	p.At = &at

	return p, true
}

/*
cleanPages returns the usable pages, the last ones kept when there are too
many, with consecutive visits to the same address counted once.
*/
func cleanPages(pages []Page, at time.Time) []Page {
	out := make([]Page, 0, len(pages))

	for _, p := range pages {
		cleaned, ok := cleanPage(p, at)
		if !ok || (len(out) > 0 && out[len(out)-1].URL == cleaned.URL) {
			continue
		}

		out = append(out, cleaned)
	}

	if len(out) > maxRecentPages {
		out = out[len(out)-maxRecentPages:]
	}

	return out
}

/*
pushPage adds the page to the recent ones, unless it repeats the latest.
*/
func pushPage(recent []Page, p Page) []Page {
	if len(recent) > 0 && recent[len(recent)-1].URL == p.URL {
		recent[len(recent)-1] = p

		return recent
	}

	return cleanPages(append(recent, p), *p.At)
}

/*
IPPrefix returns the network a visitor's address belongs to, a /24 for IPv4
and a /48 for IPv6, which is all the install keeps of it.
*/
func IPPrefix(ip string) *netip.Addr {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil
	}

	bits := 48
	if addr.Unmap().Is4() {
		addr, bits = addr.Unmap(), 24
	}

	prefix, err := addr.Prefix(bits)
	if err != nil {
		return nil
	}

	masked := prefix.Addr()

	return &masked
}

/*
cleanContext returns the page context fit to store: scalar values only, at
most thirty keys and four kilobytes, longer text cut. A null value removes
the key from what was stored before.
*/
func cleanContext(stored json.RawMessage, change map[string]any) json.RawMessage {
	merged := map[string]any{}
	_ = json.Unmarshal(stored, &merged)

	for key, value := range change {
		key = truncate(str.Clean(key), 64)
		if key == "" {
			continue
		}

		if value == nil {
			delete(merged, key)

			continue
		}

		if scalar, ok := contextValue(value); ok && (len(merged) < maxContextKeys || merged[key] != nil) {
			merged[key] = scalar
		}
	}

	encoded, err := json.Marshal(merged)
	if err != nil || len(encoded) > maxContextBytes {
		return stored
	}

	return encoded
}

/*
contextValue returns a scalar value as stored, and false for anything else.
*/
func contextValue(value any) (any, bool) {
	switch v := value.(type) {
	case string:
		return truncate(str.Clean(v), maxContextValue), true
	case bool, float64:
		return v, true
	}

	return nil, false
}

/*
truncate cuts the text to at most n bytes without splitting a character.
*/
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}

	return s[:n]
}

/*
cleanUserAgent returns the browser's description fit to store.
*/
func cleanUserAgent(ua string) *string {
	ua = truncate(strings.TrimSpace(ua), maxUserAgent)
	if ua == "" {
		return nil
	}

	return &ua
}
