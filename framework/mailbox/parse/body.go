/*
|--------------------------------------------------------------------------
| Message Bodies
|--------------------------------------------------------------------------
|
| What a person wrote can sit in several parts of a message: a mail
| client that places a file mid-message splits the text around it, and a
| list server appends its footer as a part of its own. So every part
| that is text and not a file counts, joined in the order it was written,
| and none of those parts is also listed as an attachment.
|
| Everything else is an attachment: files first, then inline parts, each
| in the order written.
|
*/

package parse

import (
	"html"
	"slices"
	"strings"

	"github.com/jhillyerd/enmime/v2"

	"github.com/dimmihq/dimmi/framework/dom"
)

/*
Media types and the disposition that decide what a part is.
*/
const (
	typePlain       = "text/plain"
	typeHTML        = "text/html"
	typeAlternative = "multipart/alternative"
	typeBinary      = "application/octet-stream"
	prefixMultipart = "multipart/"
	dispAttachment  = "attachment"
	dispInline      = "inline"
)

/*
Separators between body parts. A blank line ends a paragraph and nothing
else, so no later reading of the text can take it for a signature or a
quote, and an empty block does the same in HTML.
*/
const (
	textSeparator = "\n\n"
	htmlSeparator = "\n<div></div>\n"
)

/*
section is the body found in one branch of a message, as text, HTML or
both.
*/
type section struct {
	text, html string
}

/*
blank reports whether the section holds nothing to read.
*/
func (s section) blank() bool {
	return strings.TrimSpace(s.text) == "" && strings.TrimSpace(s.html) == ""
}

/*
collector gathers the attachments of a message while its bodies are
read, and remembers text files that may have to serve as the body.
*/
type collector struct {
	attachments []Attachment
	named       []*enmime.Part
}

/*
readBodies fills in the bodies and attachments of a parsed envelope.
*/
func readBodies(parsed *Parsed, env *enmime.Envelope) {
	body := &collector{}
	found := body.root(env)

	if found.blank() && len(body.named) > 0 {
		found = leaf(body.named[0])
	}

	parsed.Text, parsed.HTML = found.text, found.html
	parsed.Attachments = body.attachments
	slices.SortStableFunc(parsed.Attachments, filesFirst)

	if strings.TrimSpace(parsed.Text) == "" && parsed.HTML != "" {
		parsed.Text, parsed.TextDerived = dom.ToText(parsed.HTML), true
	}
}

/*
root reads the top of a message. Unlike a nested part, a top-level text
part with a file name is still the body, unless it says it is an
attachment.
*/
func (c *collector) root(env *enmime.Envelope) section {
	root := env.Root
	if root.FirstChild != nil || strings.HasPrefix(root.ContentType, prefixMultipart) {
		return c.read(root)
	}

	if slices.Contains(env.Attachments, root) || slices.Contains(env.Inlines, root) {
		c.attachments = append(c.attachments, attachment(root))

		return section{}
	}

	return leaf(root)
}

/*
read returns the body of a part, listing every file it comes across.
*/
func (c *collector) read(part *enmime.Part) section {
	if !strings.HasPrefix(part.ContentType, prefixMultipart) {
		return c.single(part)
	}

	var children []section
	for child := part.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, c.read(child))
	}

	if part.ContentType == typeAlternative {
		return alternative(children)
	}

	return sequence(children)
}

/*
single returns the body in a part that is not a container, or lists the
part as a file.
*/
func (c *collector) single(part *enmime.Part) section {
	isText := part.ContentType == typePlain || part.ContentType == typeHTML
	if isText && part.Disposition != dispAttachment {
		if part.FileName == "" {
			return leaf(part)
		}

		c.named = append(c.named, part)
	}

	c.attachments = append(c.attachments, attachment(part))

	return section{}
}

/*
leaf returns the content of a body part as text or HTML.
*/
func leaf(part *enmime.Part) section {
	if part.ContentType == typeHTML {
		return section{html: decodedContent(part)}
	}

	return section{text: decodedContent(part)}
}

/*
alternative picks from versions of the same content: the first that has
text and the first that has HTML.
*/
func alternative(versions []section) section {
	var picked section

	for _, version := range versions {
		if strings.TrimSpace(picked.text) == "" {
			picked.text = version.text
		}

		if strings.TrimSpace(picked.html) == "" {
			picked.html = version.html
		}
	}

	return picked
}

/*
sequence joins parts that follow each other. When any of them is HTML
the rest are converted, so neither form of the body is missing words the
other has.
*/
func sequence(parts []section) section {
	parts = slices.DeleteFunc(parts, section.blank)
	if len(parts) == 1 {
		return parts[0]
	}

	hasHTML := slices.ContainsFunc(parts, func(s section) bool { return s.html != "" })
	texts := make([]string, 0, len(parts))
	htmls := make([]string, 0, len(parts))

	for _, part := range parts {
		texts = append(texts, textOf(part))
		htmls = append(htmls, htmlOf(part))
	}

	joined := section{text: strings.Join(texts, textSeparator)}
	if hasHTML {
		joined.html = strings.Join(htmls, htmlSeparator)
	}

	return joined
}

func textOf(s section) string {
	if strings.TrimSpace(s.text) != "" {
		return strings.TrimRight(s.text, "\r\n")
	}

	return dom.ToText(s.html)
}

func htmlOf(s section) string {
	if s.html != "" {
		return s.html
	}

	lines := strings.ReplaceAll(strings.TrimSpace(s.text), "\r\n", "\n")

	return "<p>" + strings.ReplaceAll(html.EscapeString(lines), "\n", "<br>") + "</p>"
}

func filesFirst(a, b Attachment) int {
	switch {
	case a.Inline == b.Inline:
		return 0
	case b.Inline:
		return -1
	}

	return 1
}

/*
attachment describes a part as a file. A part counts as inline when it
says so, or when it has a content id and says nothing. A text file is
kept in UTF-8 like the rest of the text.
*/
func attachment(part *enmime.Part) Attachment {
	inline := part.Disposition == dispInline ||
		(part.Disposition == "" && part.ContentType != typeBinary && part.ContentID != "")

	content := part.Content
	if strings.HasPrefix(part.ContentType, "text/") {
		content = []byte(decodedContent(part))
	}

	return Attachment{
		Filename:    decodeWords(repairUTF8([]byte(part.FileName))),
		ContentType: part.ContentType,
		ContentID:   part.ContentID,
		Inline:      inline,
		Size:        int64(len(content)),
		Content:     content,
	}
}
