/*
|--------------------------------------------------------------------------
| Message Body
|--------------------------------------------------------------------------
|
| A received email becomes three things. The visible body is what the
| customer wrote this time, the only text teammates read first and the
| agent ever sees. It is Markdown when it came from HTML and the words
| exactly as written when the mail was plain text, because plain text
| is not Markdown: its format says which, so that a "*" or a "<div>" is
| never taken for markup. The full text of the whole message and its
| HTML, sanitized, with the quoted thread and the signature marked, are
| kept beside it for anyone who wants to read more.
|
| HTML is distrusted throughout. Before anything else, what a reader
| would never see is dropped: hidden elements, which can carry
| instructions aimed at the agent, and tracking pixels. The sanitizer
| then keeps only email-safe markup, inline styles from a short list
| and links to the web, mail, phone numbers, inline parts and embedded
| images.
|
| Mail clients mark their quoted thread in the HTML itself, so that pass
| comes first; plain text quotes are found again in the Markdown.
|
*/

package email

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/dom"
	"github.com/dimmihq/dimmi/framework/sanitize"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
maxForwardHead is how much of a quote is read for a forward's marker.
*/
const maxForwardHead = 800

/*
Size bounds of a stored body: the visible part, the HTML and the full
text.
*/
const (
	maxBodyBytes = 64 << 10
	maxHTMLBytes = 512 << 10
	maxFullBytes = 1 << 20
)

/*
Attributes that mark the quoted thread and the signature in stored
HTML.
*/
const (
	QuotedAttr    = "data-dimmi-quoted"
	SignatureAttr = "data-dimmi-signature"
)

/*
Selectors of what is not the customer's answer next to a quote, and of
quotes that wrap what they quote, as opposed to a header above it.
*/
const (
	notAnswerSelector    = "blockquote, [" + SignatureAttr + "]"
	wrappingQuoteElement = "blockquote, div.gmail_quote, div[class*=yahoo_quoted], div.protonmail_quote, " +
		"div[data-marker]"
)

var (
	hiddenStyle = regexp.MustCompile(`(?i)(display\s*:\s*none|visibility\s*:\s*hidden|mso-hide\s*:\s*all|` +
		`(^|[;\s])color\s*:\s*(transparent|rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*0(\.0+)?\s*\))|` +
		`font-size\s*:\s*(0(\.0+)?(px|pt|em|rem)?|0?\.\d+(px|pt)|0?\.0\d*(em|rem)|` +
		`1(\.0+)?(px|pt))\s*(;|!|$)|` +
		`max-height\s*:\s*0(px)?\s*(;|!|$)|opacity\s*:\s*0(\.0+)?\s*(;|!|$)|text-indent\s*:\s*-\d{3,}|` +
		`(^|[;\s])(left|top|margin-left)\s*:\s*-\d{4,})`)
	zeroBox        = regexp.MustCompile(`(?i)(^|[;\s])(height|width)\s*:\s*0(px)?\s*(;|!|$)`)
	overflowHidden = regexp.MustCompile(`(?i)overflow\s*:\s*hidden`)
	pixelBox       = regexp.MustCompile(`(?i)(^|[;\s])(height|width)\s*:\s*[01](px)?\s*(;|!|$)`)
	trailingRule   = regexp.MustCompile(`(\n\s*(\* \* \*|-{3,}|_{3,})\s*)+$`)
	blankLines     = regexp.MustCompile(`\n{3,}`)
)

/*
Body is a received message's body as stored.

Visible is the part teammates read, in Format. Full is the whole
message as plain words, quote and signature included. HTML is the full
sanitized HTML with the quote and signature marked, empty for plain
text mail. Quote, Signature and Fallback say what stripping did;
Truncated that Visible was cut at the size limit, FullTruncated that
Full was, and HTMLTooLarge that the HTML was not kept.
*/
type Body struct {
	Visible       string
	Format        enums.BodyFormat
	Full          string
	HTML          string
	Quote         bool
	Signature     bool
	Fallback      bool
	Truncated     bool
	FullTruncated bool
	HTMLTooLarge  bool
}

/*
BuildBody turns a message's HTML part, or its text part, into the stored
body. It never fails, since a message with words in it must never be
lost to its markup: HTML that cannot be read gives way to the text part,
then to the HTML with its tags stripped, and HTML that reads as nothing
gives way to the text part.
*/
func BuildBody(html, text string) Body {
	if strings.TrimSpace(html) == "" {
		return textBody(text)
	}

	body, err := htmlBody(html)
	if err != nil {
		return unreadableHTMLBody(html, text)
	}

	if strings.TrimSpace(body.Visible) == "" && strings.TrimSpace(text) != "" {
		return textBody(text)
	}

	return body
}

/*
unreadableHTMLBody builds the body of a message whose HTML cannot be
parsed, leaving out what the HTML hides from a reader.
*/
func unreadableHTMLBody(html, text string) Body {
	if strings.TrimSpace(text) != "" {
		return textBody(text)
	}

	return textBody(dom.StripTags(html, hiddenTag))
}

/*
hiddenTag reports whether a tag hides its element from a reader.
*/
func hiddenTag(tag dom.Tag) bool {
	if _, hidden := tag.Attrs["hidden"]; hidden {
		return true
	}

	return hiddenByStyle(tag.Attrs["style"])
}

/*
hiddenByStyle reports whether an inline style keeps its element from a
reader: not displayed, invisible, too small to read or pushed off the
page.
*/
func hiddenByStyle(style string) bool {
	return hiddenStyle.MatchString(style) || (zeroBox.MatchString(style) && overflowHidden.MatchString(style))
}

/*
htmlBody builds the body from HTML, with the quote and signature marked
in the stored copy and cut from the Markdown.
*/
func htmlBody(html string) (Body, error) {
	doc, err := dom.Parse(strings.NewReader(html))
	if err != nil {
		return Body{}, fmt.Errorf("read html body: %w", err)
	}

	dropHidden(doc)
	signature := markSignature(doc)
	quote := markQuote(doc)
	full := HTMLPolicy().Sanitize(doc.Find("body").HTML())

	doc.Find("[" + QuotedAttr + "]").Remove()
	doc.Find("[" + SignatureAttr + "]").Remove()
	unwrapLayoutTables(doc)

	markdown, err := dom.ToMarkdown(HTMLPolicy().Sanitize(doc.Find("body").HTML()))
	if err != nil {
		return Body{}, fmt.Errorf("convert html body: %w", err)
	}

	body := strippedBody(markdown)
	body.Visible = strings.TrimSpace(trailingRule.ReplaceAllString(body.Visible, ""))
	body.Quote, body.Signature = body.Quote || quote, body.Signature || signature

	if body.Fallback || strings.TrimSpace(body.Visible) == "" {
		body = fallbackBody(full, body)
	}

	body.Full, body.FullTruncated = capFull(strings.TrimSpace(dom.ToText(full)))
	body.HTML = full
	if len(full) > maxHTMLBytes {
		body.HTML, body.HTMLTooLarge = "", true
	}

	return body, nil
}

/*
textBody builds the body of a plain text message.
*/
func textBody(text string) Body {
	whole := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))

	body := strippedBody(whole)
	body.Format = enums.BodyFormatPlain
	body.Full, body.FullTruncated = capFull(whole)

	return body
}

/*
strippedBody runs the text pass and caps the result, as Markdown unless
the caller says otherwise.
*/
func strippedBody(markdown string) Body {
	stripped := StripText(markdown)
	visible, truncated := capBody(blankLines.ReplaceAllString(stripped.Visible, "\n\n"))

	return Body{
		Visible: visible, Format: enums.BodyFormatMarkdown, Quote: stripped.Quote, Signature: stripped.Signature,
		Fallback: stripped.Fallback, Truncated: truncated,
	}
}

/*
fallbackBody keeps the whole message when stripping the HTML left
nothing to read.
*/
func fallbackBody(full string, body Body) Body {
	markdown, err := dom.ToMarkdown(full)
	if err != nil || strings.TrimSpace(markdown) == "" {
		return body
	}

	visible, truncated := capBody(blankLines.ReplaceAllString(strings.TrimSpace(markdown), "\n\n"))

	return Body{
		Visible: visible, Format: enums.BodyFormatMarkdown, Fallback: body.Quote || body.Signature,
		Truncated: truncated,
	}
}

/*
capBody cuts a body over the size limit at the last paragraph break
before it, and says so. The thread offers the whole message beside a
body cut this way.
*/
func capBody(text string) (string, bool) {
	if len(text) <= maxBodyBytes {
		return text, false
	}

	cut := text[:maxBodyBytes]
	if i := strings.LastIndex(cut, "\n\n"); i > 0 {
		cut = cut[:i]
	}

	return strings.ToValidUTF8(cut, ""), true
}

/*
capFull cuts the full text at its size limit, and says so. The raw
email still holds the rest.
*/
func capFull(text string) (string, bool) {
	if len(text) <= maxFullBytes {
		return text, false
	}

	return strings.ToValidUTF8(text[:maxFullBytes], ""), true
}

/*
dropHidden removes what a reader never sees: elements hidden by their
style or attribute, and tracking pixels.
*/
func dropHidden(doc *dom.Document) {
	doc.Find("script, style, head, title, [hidden]").Remove()

	doc.Find("[style]").Each(func(el dom.Selection) {
		if style, _ := el.Attr("style"); hiddenByStyle(style) {
			el.Remove()
		}
	})

	doc.Find("img").Each(func(img dom.Selection) {
		if pixelImage(img) {
			img.Remove()
		}
	})
}

/*
unwrapLayoutTables replaces every table of a single column, which only
lays a mail out, with its content. Inner tables go first, so an outer
one takes their content along.
*/
func unwrapLayoutTables(doc *dom.Document) {
	var tables []dom.Selection

	doc.Find("table").Each(func(table dom.Selection) { tables = append(tables, table) })

	for _, table := range slices.Backward(tables) {
		if table.Find("td + td, td + th, th + td, th + th").Len() > 0 {
			continue
		}

		var content strings.Builder

		table.Find("td, th").Each(func(cell dom.Selection) {
			content.WriteString("<div>" + cell.HTML() + "</div>")
		})

		table.ReplaceWithHTML(content.String())
	}
}

/*
pixelImage reports whether an image is a pixel or smaller, by its size
attributes or its style.
*/
func pixelImage(img dom.Selection) bool {
	for _, name := range []string{"width", "height"} {
		value, _ := img.Attr(name)
		if size := strings.TrimSuffix(strings.TrimSpace(value), "px"); size == "0" || size == "1" {
			return true
		}
	}

	style, _ := img.Attr("style")

	return pixelBox.MatchString(style)
}

/*
markQuote marks the first quoted thread a mail client wrapped, and
everything after it, unless it holds a forward, which is content, or
the customer answered inside or below it. It reports whether it marked
one.
*/
func markQuote(doc *dom.Document) bool {
	marked := false

	doc.Find(quoteSelectors()).Each(func(el dom.Selection) {
		if marked {
			return
		}

		marked = true
		if forwardedQuote(el) || answeredInside(el) {
			return
		}

		el.SetAttr(QuotedAttr, "")
		el.NextAll().SetAttr(QuotedAttr, "")
	})

	return doc.Find("["+QuotedAttr+"]").Len() > 0
}

/*
forwardedQuote reports whether what looks like a quote opens with a
forward's marker, or a header whose subject says it was forwarded.
*/
func forwardedQuote(el dom.Selection) bool {
	head := []rune(strings.ReplaceAll(el.Text()+" "+el.TextAfter(""), "\u00a0", " "))

	return forwardedHead.MatchString(string(head[:min(len(head), maxForwardHead)]))
}

/*
answeredInside reports whether the customer wrote below a quote, or
between the quoted parts inside it, as in an inline reply.
*/
func answeredInside(el dom.Selection) bool {
	if el.Is(wrappingQuoteElement) && answeredAfter(el) {
		return true
	}

	answered := false

	el.Find("blockquote").Each(func(quote dom.Selection) {
		if !quote.Inside("blockquote") && answeredAfter(quote) {
			answered = true
		}
	})

	return answered
}

/*
answeredAfter reports whether a quote is followed by text of the
customer's own: not more quotes, not a signature and not a footer.
*/
func answeredAfter(quote dom.Selection) bool {
	return !isFooter(strings.Split(quote.TextAfter(notAnswerSelector), "\n"))
}

/*
markSignature marks every signature a mail client wrapped, and reports
whether there was one.
*/
func markSignature(doc *dom.Document) bool {
	signatures := doc.Find(`div.gmail_signature, div[data-smartmail="gmail_signature"], #Signature, ` +
		`div.moz-signature, #ms-outlook-mobile-signature`)
	signatures.SetAttr(SignatureAttr, "")

	return signatures.Len() > 0
}

/*
quoteSelectors matches the quoted thread of the mail clients that wrap
it: Gmail, Apple Mail and Thunderbird, Outlook on the web and desktop,
Yahoo, Proton, Zimbra and Zoho.
*/
func quoteSelectors() string {
	return strings.Join([]string{
		"div.gmail_quote", `blockquote[type="cite"]`, "div#divRplyFwdMsg", "div#appendonsend",
		"hr#stopSpelling", `div[class*="yahoo_quoted"]`, "div.protonmail_quote", "blockquote.protonmail_quote",
		"div.moz-cite-prefix", `div[data-marker="__QUOTED_TEXT__"]`, `div[style*="border-top:solid #E1E1E1"]`,
		"#zwchr",
	}, ", ")
}

/*
HTMLPolicy returns the sanitizer policy for received HTML: the markup
mail is written in, a short list of inline styles, and links to the
web, mail, phone numbers, inline parts and embedded images only.
*/
func HTMLPolicy() *sanitize.Policy {
	p := sanitize.NewPolicy().
		AllowElements("a", "b", "i", "u", "s", "em", "strong", "p", "br", "div", "span", "blockquote", "ul",
			"ol", "li", "table", "thead", "tbody", "tfoot", "tr", "td", "th", "h1", "h2", "h3", "h4", "h5", "h6",
			"pre", "code", "hr", "img", "font", "center", "small", "sub", "sup", "strike", "del", "ins", "caption").
		AllowURLSchemes("http", "https", "mailto", "tel", "cid").
		AllowDataURIImages()

	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt", "width", "height").OnElements("img")
	p.AllowAttrs("colspan", "rowspan", "align", "valign").OnElements("td", "th")
	p.AllowAttrs("type").OnElements("blockquote")
	p.AllowAttrs(QuotedAttr, SignatureAttr, "dir").Globally()
	p.AllowStyles("color", "background-color", "font-weight", "font-style", "text-decoration", "text-align",
		"font-size", "font-family", "margin", "padding", "border", "width").Globally()

	return p
}
