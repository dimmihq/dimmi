/*
|--------------------------------------------------------------------------
| Notion Markdown
|--------------------------------------------------------------------------
|
| Each block type has one Markdown form. Styled text keeps its bold,
| italic, strikethrough, inline code and links; anything without a
| Markdown form keeps its text so nothing a page says is lost.
|
*/

package notion

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
decodeBlock reads one block of the API's answer.
*/
func decodeBlock(raw map[string]json.RawMessage) block {
	var b block

	_ = json.Unmarshal(raw["id"], &b.ID)
	_ = json.Unmarshal(raw["type"], &b.Type)
	_ = json.Unmarshal(raw["has_children"], &b.HasChildren)
	_ = json.Unmarshal(raw[b.Type], &b.body)

	return b
}

/*
render writes one block as Markdown, and the picture it shows, if any.
*/
func render(b block, number int) (string, *sources.Asset) {
	text := inline(b.body.RichText)

	switch b.Type {
	case "paragraph":
		return text, nil
	case "heading_1", "heading_2", "heading_3":
		return strings.Repeat("#", int(b.Type[len(b.Type)-1]-'0')+1) + " " + text, nil
	case "bulleted_list_item", "toggle":
		return "- " + text, nil
	case "numbered_list_item":
		return strconv.Itoa(number) + ". " + text, nil
	case "to_do":
		return checkbox(b.body) + text, nil
	case "quote", "callout":
		return "> " + strings.ReplaceAll(text, "\n", "\n> "), nil
	case "code":
		return "```" + b.body.Language + "\n" + plain(b.body.RichText) + "\n```", nil
	case "divider":
		return "---", nil
	case "image":
		return image(b.body)
	case "bookmark", "embed", "link_preview":
		if b.body.URL == "" {
			return "", nil
		}

		return "[" + b.body.URL + "](" + b.body.URL + ")", nil
	}

	return text, nil
}

/*
image writes a picture block and names its file.
*/
func image(c blockContent) (string, *sources.Asset) {
	src := c.External.URL
	if src == "" {
		src = c.File.URL
	}

	if src == "" {
		return "", nil
	}

	alt := plain(c.Caption)

	return "![" + alt + "](" + src + ")", &sources.Asset{URL: src, Name: alt}
}

/*
checkbox is a to-do's box, ticked when it is done.
*/
func checkbox(c blockContent) string {
	if c.Checked {
		return "- [x] "
	}

	return "- [ ] "
}

/*
inline writes styled text as Markdown.
*/
func inline(runs []richText) string {
	var b strings.Builder

	for _, r := range runs {
		t := r.PlainText
		if strings.TrimSpace(t) == "" {
			b.WriteString(t)

			continue
		}

		a := r.Annotations

		switch {
		case a.Code:
			t = "`" + t + "`"
		case a.Bold && a.Italic:
			t = "***" + t + "***"
		case a.Bold:
			t = "**" + t + "**"
		case a.Italic:
			t = "_" + t + "_"
		}

		if a.Strike {
			t = "~~" + t + "~~"
		}

		if r.Href != "" {
			t = "[" + t + "](" + r.Href + ")"
		}

		b.WriteString(t)
	}

	return b.String()
}

/*
plain joins the text of the runs without styles.
*/
func plain(runs []richText) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.PlainText)
	}

	return b.String()
}

/*
shiftHeadings moves a page's headings so its top level reads as a
second-level heading, the level a document's sections are cut at, and
the levels under it keep their order. Code blocks are left alone.
*/
func shiftHeadings(md string) string {
	lines := strings.Split(md, "\n")
	top := 0

	walkHeadings(lines, func(_, level int) {
		if top == 0 || level < top {
			top = level
		}
	})

	if top == 0 || top == 2 {
		return md
	}

	walkHeadings(lines, func(i, level int) {
		next := min(max(level-top+2, 2), 6)
		lines[i] = strings.Repeat("#", next) + lines[i][level:]
	})

	return strings.Join(lines, "\n")
}

/*
walkHeadings calls fn with each heading line's index and level,
skipping fenced code.
*/
func walkHeadings(lines []string, fn func(i, level int)) {
	fenced := false

	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced

			continue
		}

		level := len(line) - len(strings.TrimLeft(line, "#"))
		if !fenced && level > 0 && level <= 6 && strings.HasPrefix(line[level:], " ") {
			fn(i, level)
		}
	}
}
