/*
|--------------------------------------------------------------------------
| Slack To Markdown
|--------------------------------------------------------------------------
|
| A message written in Slack becomes Markdown in the inbox. Its rich
| text blocks are read first, since they say exactly what is bold, a
| list, a quote or code; the plain mrkdwn text is the fallback for a
| message without them. Mentions become the person's name and channels
| their name, so the inbox shows what the customer saw. A link becomes a
| Markdown link only for web, mail and phone addresses; anything else
| stays plain text, so a crafted link can never run script in the inbox.
|
*/

package slack

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

/*
Names resolves a Slack user id to the name the inbox shows, or empty
when unknown.
*/
type Names func(userID string) string

/*
richBlock is one block of a message, with its rich text elements.
*/
type richBlock struct {
	Type     string        `json:"type"`
	Elements []richElement `json:"elements"`
}

/*
richElement is one element of rich text: a section, list, quote or
code, or a run of text, a link, a mention or an emoji inside one.
*/
type richElement struct {
	Type      string          `json:"type"`
	Elements  []richElement   `json:"elements"`
	Text      string          `json:"text"`
	URL       string          `json:"url"`
	UserID    string          `json:"user_id"`
	ChannelID string          `json:"channel_id"`
	Name      string          `json:"name"`
	Range     string          `json:"range"`
	Style     json.RawMessage `json:"style"`
	Indent    int             `json:"indent"`
}

/*
textStyle is how a run of text is styled.
*/
type textStyle struct {
	Bold   bool `json:"bold"`
	Italic bool `json:"italic"`
	Strike bool `json:"strike"`
	Code   bool `json:"code"`
}

/*
ToMarkdown converts a Slack message to Markdown, from its rich text
blocks when it has them, else from its mrkdwn text.
*/
func ToMarkdown(text string, blocks json.RawMessage, names Names) string {
	var parsed []richBlock
	if len(blocks) > 0 && json.Unmarshal(blocks, &parsed) == nil {
		if out := richMarkdown(parsed, names); out != "" {
			return out
		}
	}

	return MrkdwnToMarkdown(text, names)
}

/*
richMarkdown renders the rich text blocks, one paragraph each.
*/
func richMarkdown(blocks []richBlock, names Names) string {
	var parts []string

	for _, block := range blocks {
		if block.Type != "rich_text" {
			continue
		}

		for _, el := range block.Elements {
			if rendered := strings.TrimRight(richPart(el, names), "\n"); rendered != "" {
				parts = append(parts, rendered)
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

/*
richPart renders one top-level rich text element.
*/
func richPart(el richElement, names Names) string {
	switch el.Type {
	case "rich_text_list":
		return richList(el, names)
	case "rich_text_quote":
		return "> " + strings.ReplaceAll(inline(el.Elements, names), "\n", "\n> ")
	case "rich_text_preformatted":
		return "```\n" + plain(el.Elements) + "\n```"
	default:
		return inline(el.Elements, names)
	}
}

/*
richList renders a list, ordered or not, at its indent.
*/
func richList(el richElement, names Names) string {
	var style string
	_ = json.Unmarshal(el.Style, &style)

	indent := strings.Repeat("  ", el.Indent)
	lines := make([]string, 0, len(el.Elements))

	for i, item := range el.Elements {
		marker := "- "
		if style == "ordered" {
			marker = strconv.Itoa(i+1) + ". "
		}

		lines = append(lines, indent+marker+inline(item.Elements, names))
	}

	return strings.Join(lines, "\n")
}

/*
inline renders a run of inline elements.
*/
func inline(elements []richElement, names Names) string {
	var b strings.Builder

	for _, el := range elements {
		b.WriteString(inlineOne(el, names))
	}

	return b.String()
}

/*
inlineOne renders one inline element.
*/
func inlineOne(el richElement, names Names) string {
	switch el.Type {
	case "text":
		return styled(el.Text, el.Style)
	case "link":
		return link(el.URL, el.Text)
	case "user":
		return "@" + nameOf(el.UserID, names)
	case "channel":
		return "#" + el.ChannelID
	case "emoji":
		return ":" + el.Name + ":"
	case "broadcast":
		return "@" + el.Range
	default:
		return el.Text
	}
}

/*
styled wraps a run of text in the Markdown of its style.
*/
func styled(text string, raw json.RawMessage) string {
	var style textStyle
	if len(raw) == 0 || json.Unmarshal(raw, &style) != nil || strings.TrimSpace(text) == "" {
		return text
	}

	if style.Code {
		return "`" + text + "`"
	}

	marks := ""
	if style.Bold {
		marks += "**"
	}

	if style.Italic {
		marks += "*"
	}

	if style.Strike {
		marks += "~~"
	}

	return marks + text + reverse(marks)
}

/*
plain renders inline elements without styling, for code.
*/
func plain(elements []richElement) string {
	var b strings.Builder

	for _, el := range elements {
		switch {
		case el.Type == "link" && el.Text == "":
			b.WriteString(el.URL)
		default:
			b.WriteString(el.Text)
		}
	}

	return b.String()
}

/*
safeScheme reports whether a link may become a Markdown link.
*/
func safeScheme(url string) bool {
	lower := strings.ToLower(url)
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:"} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}

	return false
}

/*
link renders a link, as plain text when its scheme is not safe.
*/
func link(url, label string) string {
	if !safeScheme(url) {
		if label != "" {
			return label
		}

		return url
	}

	if label == "" || label == url {
		return url
	}

	return "[" + label + "](" + url + ")"
}

/*
nameOf returns the user's name, or their id when unknown.
*/
func nameOf(userID string, names Names) string {
	if names != nil {
		if name := names(userID); name != "" {
			return name
		}
	}

	return userID
}

/*
The patterns of Slack's mrkdwn markup.
*/
var (
	angledPattern = regexp.MustCompile(`<([^<>|]+)(?:\|([^<>]*))?>`)
	boldPattern   = regexp.MustCompile(`(^|[\s(])\*([^*\n]+)\*`)
	italicPattern = regexp.MustCompile(`(^|[\s(])_([^_\n]+)_`)
	strikePattern = regexp.MustCompile(`(^|[\s(])~([^~\n]+)~`)
)

/*
MrkdwnToMarkdown converts Slack's mrkdwn text to Markdown.
*/
func MrkdwnToMarkdown(text string, names Names) string {
	out := angledPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := angledPattern.FindStringSubmatch(match)

		return angled(parts[1], parts[2], names)
	})

	out = italicPattern.ReplaceAllString(out, "$1\x00$2\x00")
	out = boldPattern.ReplaceAllString(out, "$1**$2**")
	out = strikePattern.ReplaceAllString(out, "$1~~$2~~")
	out = strings.ReplaceAll(out, "\x00", "*")

	return strings.TrimSpace(html.UnescapeString(out))
}

/*
angled renders one of mrkdwn's angled references: a mention, a channel,
a special mention or a link.
*/
func angled(target, label string, names Names) string {
	switch {
	case strings.HasPrefix(target, "@"):
		return "@" + nameOf(strings.TrimPrefix(target, "@"), names)
	case strings.HasPrefix(target, "#"):
		if label != "" {
			return "#" + label
		}

		return target
	case strings.HasPrefix(target, "!"):
		return "@" + strings.TrimPrefix(strings.SplitN(target, "^", 2)[0], "!")
	default:
		return link(target, label)
	}
}

/*
reverse returns the marks in the order that closes them.
*/
func reverse(marks string) string {
	runes := []rune(marks)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}
