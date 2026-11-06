/*
|--------------------------------------------------------------------------
| Link Guard
|--------------------------------------------------------------------------
|
| The model writes links; only the ones the workspace trusts reach a
| customer. A link is trusted when its host belongs to a source the
| answer was built from or to a domain the workspace allowlisted.
| Everything else loses its URL: a Markdown link keeps its text, a bare
| URL disappears. Images and raw HTML never pass, whatever they point
| at, because a reply is text and a phishing page needs only one click.
|
*/

package linkguard

import (
	"net/url"
	"regexp"
	"strings"
)

/*
Guard holds the compiled patterns, built once and safe for concurrent
use.
*/
type Guard struct {
	image    *regexp.Regexp
	mdLink   *regexp.Regexp
	autolink *regexp.Regexp
	html     *regexp.Regexp
	bare     *regexp.Regexp
	spaces   *regexp.Regexp
}

/*
New compiles the patterns.
*/
func New() *Guard {
	return &Guard{
		image:    regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`),
		mdLink:   regexp.MustCompile(`\[([^\]]*)\]\(\s*([^)\s]+)(?:\s+"[^"]*")?\s*\)`),
		autolink: regexp.MustCompile(`(?i)<((?:https?|mailto|ftp):[^>\s]+)>`),
		html:     regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*)?/?>`),
		spaces:   regexp.MustCompile(`(\S)[ \t]{2,}`),
		bare: regexp.MustCompile(`(?i)\b(?:(?:https?|ftp|javascript|mailto):(?://)?|www\.)` +
			`[^\s<>()\[\]"']+`),
	}
}

/*
Filter returns the text with every untrusted link removed. Allowed
entries are hosts ("help.example.com") or wildcard domains
("*.example.com", which also covers the bare domain).
*/
func (g *Guard) Filter(text string, allowed []string) string {
	out := g.image.ReplaceAllString(text, "")
	out = g.autolink.ReplaceAllString(out, "$1")

	var kept []string
	out = g.mdLink.ReplaceAllStringFunc(out, func(m string) string {
		parts := g.mdLink.FindStringSubmatch(m)
		if !Allowed(parts[2], allowed) {
			return parts[1]
		}
		kept = append(kept, m)
		return placeholder(len(kept) - 1)
	})

	out = g.html.ReplaceAllString(out, "")
	out = g.bare.ReplaceAllStringFunc(out, func(m string) string {
		link, trail := splitTrailing(m)
		if Allowed(link, allowed) {
			return link + trail
		}
		return trail
	})

	for i, link := range kept {
		out = strings.Replace(out, placeholder(i), link, 1)
	}

	return g.tidy(out)
}

/*
Allowed reports whether a URL may reach a customer: an https or http
link, or a mailto address, whose host matches the allowlist.
*/
func Allowed(raw string, allowed []string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}

	var host string
	switch strings.ToLower(u.Scheme) {
	case "https", "http":
		host = u.Hostname()
	case "mailto":
		_, domain, ok := strings.Cut(u.Opaque, "@")
		if !ok {
			return false
		}
		host = domain
	default:
		return false
	}

	return HostAllowed(host, allowed)
}

/*
HostAllowed matches one host against the allowlist, ignoring case: an
exact entry matches itself, "*.example.com" matches example.com and
every subdomain.
*/
func HostAllowed(host string, allowed []string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false
	}

	for _, entry := range allowed {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if domain, ok := strings.CutPrefix(entry, "*."); ok {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				return true
			}
			continue
		}
		if entry != "" && host == entry {
			return true
		}
	}
	return false
}

/*
Hosts returns the hosts of the URLs, skipping any that do not parse,
for building the allowlist from an answer's sources.
*/
func Hosts(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		out = append(out, strings.ToLower(u.Hostname()))
	}
	return out
}

/*
splitTrailing keeps sentence punctuation after a bare URL out of the
URL itself.
*/
func splitTrailing(m string) (string, string) {
	end := len(m)
	for end > 0 && strings.ContainsRune(".,;:!?", rune(m[end-1])) {
		end--
	}
	return m[:end], m[end:]
}

/*
placeholder stands in for a kept Markdown link while bare URLs are
filtered, so the link's own URL is never treated as bare.
*/
func placeholder(i int) string {
	return "\x00link" + string(rune('a'+i%26)) + strings.Repeat("~", i/26) + "\x00"
}

/*
tidy collapses the double spaces removed links leave inside a line,
keeping indentation, and trims line ends.
*/
func (g *Guard) tidy(s string) string {
	s = g.spaces.ReplaceAllString(s, "$1 ")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
