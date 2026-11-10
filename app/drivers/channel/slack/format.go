/*
|--------------------------------------------------------------------------
| Markdown For Slack
|--------------------------------------------------------------------------
|
| Replies are written in Markdown and Slack reads standard Markdown in a
| message's markdown text, so the text mostly goes as it is. Three things
| change on the way. Angle brackets and ampersands are escaped, so no
| text, least of all a customer's words quoted back or an agent's answer,
| can ever ping a whole channel with <!channel> or <!here> or mention
| someone. Tables, which Slack cannot show, become a code block that
| keeps their columns. Pictures become links, since a reply's files go as
| real files.
|
*/

package slack

import (
	"regexp"
	"strings"
)

/*
MaxBodyChars is the longest markdown text one Slack message takes.
*/
const MaxBodyChars = 12000

/*
imagePattern matches a Markdown picture.
*/
var imagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)

/*
escaper turns the characters Slack reads as markup into entities.
*/
func escaper() *strings.Replacer {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
}

/*
ToSlack renders Markdown as the markdown text of a Slack message.
*/
func ToSlack(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	esc := escaper()
	inFence, inTable := false, false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fence := strings.HasPrefix(trimmed, "```")
		table := !inFence && strings.HasPrefix(trimmed, "|")

		switch {
		case table && !inTable:
			out = append(out, "```")
		case !table && inTable:
			out = append(out, "```")
		}

		inTable = table

		if fence {
			inFence = !inFence
		}

		if !inFence && !fence && !table {
			line = imagePattern.ReplaceAllString(line, "[$1]($2)")
		}

		out = append(out, esc.Replace(line))
	}

	if inTable {
		out = append(out, "```")
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

/*
Truncate cuts the text to at most limit characters, ending it with an
ellipsis when it was longer.
*/
func Truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit-1]) + "…"
}
