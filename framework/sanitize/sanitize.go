/*
|--------------------------------------------------------------------------
| HTML Sanitizer
|--------------------------------------------------------------------------
|
| Any HTML that did not come from our own templates is hostile until a
| policy has cleaned it: messages people write, email bodies, imported
| pages and everything a language model produces. A policy is an
| allowlist, so an element, attribute, style or URL scheme nobody named
| never reaches a browser.
|
| Build a policy once at startup and share it. Sanitize is safe to call
| from many goroutines; the builder methods are not, so they all run
| before the first Sanitize.
|
*/

package sanitize

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

/*
safeRel are the link relations user content may set on its own.
Each one only takes privileges away from the link.
*/
const safeRel = `^(?:nofollow|noopener|noreferrer|ugc)(?:\s+(?:nofollow|noopener|noreferrer|ugc))*$`

/*
Policy is an allowlist of elements, attributes, styles and URL schemes.
*/
type Policy struct {
	policy *bluemonday.Policy
}

/*
NewPolicy returns an empty policy that keeps only the escaped text of a
document. Allow what a use needs on top of it.
*/
func NewPolicy() *Policy {
	return &Policy{policy: bluemonday.NewPolicy()}
}

/*
Strict returns a policy that removes every element and keeps the escaped
text, for content that must never carry markup, such as names and
subjects.
*/
func Strict() *Policy {
	return &Policy{policy: bluemonday.StrictPolicy()}
}

/*
UGC returns the policy for rich text people write: headings, emphasis,
lists, tables, quotes, code, images and links, but no scripts, styles,
classes, forms or frames.

Links and images may be relative or use http, https, mailto or tel. A
link that names another host gets rel="nofollow" so the site never
vouches for it; relative links stay followed. A rel that only takes
privileges away (nofollow, noopener, noreferrer, ugc) is kept.
*/
func UGC() *Policy {
	p := bluemonday.UGCPolicy()
	p.AllowURLSchemes("tel")
	p.RequireNoFollowOnLinks(false)
	p.RequireNoFollowOnFullyQualifiedLinks(true)
	p.AllowAttrs("rel").Matching(regexp.MustCompile(safeRel)).OnElements("a")

	return &Policy{policy: p}
}

/*
Sanitize returns the HTML with everything outside the policy removed.
The result cannot run script, but it is not rebalanced: an element left
open in the input stays open, so render it inside a container of its
own.
*/
func (p *Policy) Sanitize(html string) string {
	return p.policy.Sanitize(html)
}

/*
AllowElements allows the named elements, without attributes.
*/
func (p *Policy) AllowElements(names ...string) *Policy {
	p.policy.AllowElements(names...)

	return p
}

/*
AllowURLSchemes allows links and sources with the given URL schemes,
such as "https" or "cid". URLs that do not parse are always removed.
*/
func (p *Policy) AllowURLSchemes(schemes ...string) *Policy {
	p.policy.AllowURLSchemes(schemes...)

	return p
}

/*
AllowDataURIImages allows images embedded as base64 data URIs of the
common raster types. The decoded bytes must really be such an image.
*/
func (p *Policy) AllowDataURIImages() *Policy {
	p.policy.AllowDataURIImages()

	return p
}

/*
AllowAttrs starts a rule allowing the named attributes. The rule takes
effect once OnElements or Globally says where.
*/
func (p *Policy) AllowAttrs(names ...string) *AttrRule {
	return &AttrRule{policy: p, names: names}
}

/*
AllowStyles starts a rule allowing the named inline CSS properties. The
rule takes effect once OnElements or Globally says where.
*/
func (p *Policy) AllowStyles(properties ...string) *StyleRule {
	return &StyleRule{policy: p, properties: properties}
}

/*
AttrRule allows attributes on elements, optionally only with values
matching a pattern.
*/
type AttrRule struct {
	policy  *Policy
	names   []string
	pattern *regexp.Regexp
}

/*
Matching keeps the attributes only when their whole value matches the
pattern. Anchor it with ^ and $.
*/
func (r *AttrRule) Matching(pattern *regexp.Regexp) *AttrRule {
	r.pattern = pattern

	return r
}

/*
OnElements allows the attributes on the named elements, which are
allowed along with them.
*/
func (r *AttrRule) OnElements(elements ...string) *Policy {
	rule := r.policy.policy.AllowAttrs(r.names...)
	if r.pattern != nil {
		rule.Matching(r.pattern)
	}

	rule.OnElements(elements...)

	return r.policy
}

/*
Globally allows the attributes on every allowed element.
*/
func (r *AttrRule) Globally() *Policy {
	rule := r.policy.policy.AllowAttrs(r.names...)
	if r.pattern != nil {
		rule.Matching(r.pattern)
	}

	rule.Globally()

	return r.policy
}

/*
StyleRule allows inline CSS properties on elements, optionally only with
values matching a pattern.
*/
type StyleRule struct {
	policy     *Policy
	properties []string
	pattern    *regexp.Regexp
}

/*
Matching keeps the properties only when their whole value matches the
pattern. Without it the library's own per-property checks apply.
*/
func (r *StyleRule) Matching(pattern *regexp.Regexp) *StyleRule {
	r.pattern = pattern

	return r
}

/*
OnElements allows the properties in the style attribute of the named
elements, which are allowed along with them.
*/
func (r *StyleRule) OnElements(elements ...string) *Policy {
	rule := r.policy.policy.AllowStyles(r.properties...)
	if r.pattern != nil {
		rule.Matching(r.pattern)
	}

	rule.OnElements(elements...)

	return r.policy
}

/*
Globally allows the properties in the style attribute of every allowed
element.
*/
func (r *StyleRule) Globally() *Policy {
	rule := r.policy.policy.AllowStyles(r.properties...)
	if r.pattern != nil {
		rule.Matching(r.pattern)
	}

	rule.Globally()

	return r.policy
}
