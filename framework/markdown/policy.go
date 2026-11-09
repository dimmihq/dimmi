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
| checkboxes of task lists. Anything else in the output is removed.
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
	classPattern     = `^(?:(?:alert|chroma|footnote|image)[\w-]*)(?:\s+(?:alert|chroma|footnote|image)[\w-]*)*$`
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
		AllowAttrs("checked", "disabled").Matching(regexp.MustCompile(`^$`)).OnElements("input")
}
