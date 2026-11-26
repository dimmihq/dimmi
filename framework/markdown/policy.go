/*
|--------------------------------------------------------------------------
| Default Policy
|--------------------------------------------------------------------------
|
| The sanitize policy rendered Markdown passes through unless the caller
| brings its own: the user content policy plus exactly the markup this
| package writes itself. Heading ids in any script, the alert, footnote
| and highlighting classes, the layout classes of pictures, the roles
| alerts and footnotes carry, lazy image loading, and the disabled
| checkboxes of task lists, and the article blocks: frames only from the
| allowed embed providers' players with their fixed attributes, details
| and summaries, centered text, and figures with captions. Anything else in the output is removed.
|
*/

package markdown

import (
	"regexp"

	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
Patterns for the attribute values this package writes.
*/
const (
	headingIDPattern = `^[\p{L}\p{N}_-]+$`
	classWord        = `(?:align|alert|button|chroma|details|embed|footnote|image)[\w-]*`
	classPattern     = `^` + classWord + `(?:\s+` + classWord + `)*$`
	rolePattern      = `^(?:note|doc-noteref|doc-endnotes|doc-backlink)$`
)

/*
Policy returns a new copy of the default policy. Extend it, then pass it
as Options.Sanitize.
*/
func Policy() *sanitize.Policy {
	return sanitize.UGC().
		AllowAttrs("id").Matching(regexp.MustCompile(headingIDPattern)).OnElements("h1", "h2", "h3", "h4", "h5", "h6").
		AllowAttrs("class").Matching(regexp.MustCompile(classPattern)).Globally().
		AllowAttrs("role").Matching(regexp.MustCompile(rolePattern)).Globally().
		AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("img").
		AllowAttrs("decoding").Matching(regexp.MustCompile(`^async$`)).OnElements("img").
		AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input").
		AllowAttrs("checked", "disabled").Matching(regexp.MustCompile(`^$`)).OnElements("input").
		AllowElements("details", "summary", "figure", "figcaption").
		AllowAttrs("src").Matching(playerPattern()).OnElements("iframe").
		AllowAttrs("title").OnElements("iframe").
		AllowAttrs("loading").Matching(exactly("lazy")).OnElements("iframe").
		AllowAttrs("allow").Matching(exactly(embedAllow)).OnElements("iframe").
		AllowAttrs("allowfullscreen").Matching(regexp.MustCompile(`^$`)).OnElements("iframe").
		AllowAttrs("sandbox").Matching(exactly(embedSandbox)).OnElements("iframe").
		AllowAttrs("referrerpolicy").Matching(exactly(embedReferrer)).OnElements("iframe")
}

/*
exactly matches one fixed value.
*/
func exactly(value string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(value) + `$`)
}
