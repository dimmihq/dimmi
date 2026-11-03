/*
|--------------------------------------------------------------------------
| Message Body
|--------------------------------------------------------------------------
|
| A received email becomes two things. The visible body is Markdown of
| what the customer wrote this time, the only text teammates read first
| and the agent ever sees. The full HTML is kept beside it, sanitized,
| with the quoted thread and the signature marked rather than removed.
|
| HTML is distrusted throughout. Before anything else, what a reader
| would never see is dropped: hidden elements, which can carry
| instructions aimed at the agent, and tracking pixels. The sanitizer
| then keeps only email-safe markup, inline styles from a short list
| and links to the web, mail, inline parts and embedded images.
|
| Mail clients mark their quoted thread in the HTML itself, so that pass
| comes first; plain text quotes are found again in the Markdown.
|
*/

package email

import (
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/dom"
	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
Size bounds of a stored body: Markdown and HTML.
*/
const (
	maxBodyBytes = 64 << 10
	maxHTMLBytes = 512 << 10
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
truncatedNote ends a body cut at the size limit.
*/
const truncatedNote = "\n\n[Message truncated. View original.]"

var (
	hiddenStyle = regexp.MustCompile(`(?i)(display\s*:\s*none|visibility\s*:\s*hidden|` +
		`font-size\s*:\s*0(px|pt|em)?\s*(;|$)|max-height\s*:\s*0(px)?\s*(;|$)|opacity\s*:\s*0(\.0+)?\s*(;|$))`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

/*
Body is a received message's body as stored.

Markdown is the visible part, HTML the full sanitized HTML with the
quote and signature marked, empty for plain text mail. Quote, Signature
and Fallback say what stripping did; Truncated that the Markdown was
cut at the size limit, HTMLTooLarge that the HTML was not kept.
*/
type Body struct {
	Markdown     string
	HTML         string
	Quote        bool
	Signature    bool
	Fallback     bool
	Truncated    bool
	HTMLTooLarge bool
}

/*
BuildBody turns a message's HTML part, or its text part when it has no
HTML, into the stored body.
*/
func BuildBody(html, text string) (Body, error) {
	if strings.TrimSpace(html) == "" {
		return textBody(text), nil
	}

	doc, err := dom.Parse(strings.NewReader(html))
	if err != nil {
		return Body{}, err
	}

	dropHidden(doc)
	quote := markQuote(doc)
	signature := markSignature(doc)
	full := HTMLPolicy().Sanitize(doc.Find("body").HTML())

	doc.Find("[" + QuotedAttr + "]").Remove()
	doc.Find("[" + SignatureAttr + "]").Remove()

	markdown, err := dom.ToMarkdown(HTMLPolicy().Sanitize(doc.Find("body").HTML()))
	if err != nil {
		return Body{}, err
	}

	body := strippedBody(markdown)
	body.Quote, body.Signature = body.Quote || quote, body.Signature || signature

	if body.Fallback || strings.TrimSpace(body.Markdown) == "" {
		body = fallbackBody(full, body)
	}

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
	return strippedBody(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")))
}

/*
strippedBody runs the text pass and caps the result.
*/
func strippedBody(markdown string) Body {
	stripped := StripText(markdown)
	visible, truncated := capBody(blankLines.ReplaceAllString(stripped.Visible, "\n\n"))

	return Body{
		Markdown: visible, Quote: stripped.Quote, Signature: stripped.Signature, Fallback: stripped.Fallback,
		Truncated: truncated,
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

	return Body{Markdown: visible, Fallback: body.Quote || body.Signature, Truncated: truncated}
}

/*
capBody cuts a body over the size limit at the last paragraph break
before it, and notes the cut.
*/
func capBody(markdown string) (string, bool) {
	if len(markdown) <= maxBodyBytes {
		return markdown, false
	}

	cut := markdown[:maxBodyBytes-len(truncatedNote)]
	if i := strings.LastIndex(cut, "\n\n"); i > 0 {
		cut = cut[:i]
	}

	return strings.ToValidUTF8(cut, "") + truncatedNote, true
}

/*
dropHidden removes what a reader never sees: elements hidden by their
style or attribute, and tracking pixels.
*/
func dropHidden(doc *dom.Document) {
	doc.Find("script, style, head, title, [hidden]").Remove()

	doc.Find("[style]").Each(func(el dom.Selection) {
		if style, _ := el.Attr("style"); hiddenStyle.MatchString(style) {
			el.Remove()
		}
	})

	doc.Find("img").Each(func(img dom.Selection) {
		width, _ := img.Attr("width")
		height, _ := img.Attr("height")

		if strings.TrimSpace(width) == "1" || strings.TrimSpace(height) == "1" ||
			strings.TrimSpace(width) == "0" || strings.TrimSpace(height) == "0" {
			img.Remove()
		}
	})
}

/*
markQuote marks the first quoted thread a mail client wrapped, and
everything after it, unless the customer answered below a cited quote,
as in an inline reply. It reports whether it marked one.
*/
func markQuote(doc *dom.Document) bool {
	marked := false

	doc.Find(quoteSelectors()).Each(func(el dom.Selection) {
		if marked {
			return
		}

		marked = true
		if kind, _ := el.Attr("type"); kind == "cite" && answeredAfter(el) {
			return
		}

		el.SetAttr(QuotedAttr, "")
		el.NextAll().SetAttr(QuotedAttr, "")
	})

	return doc.Find("["+QuotedAttr+"]").Len() > 0
}

/*
answeredAfter reports whether a quote is followed by text of the
customer's own, other than more quotes, which makes it an inline reply.
*/
func answeredAfter(quote dom.Selection) bool {
	answered := false

	quote.NextAll().Each(func(el dom.Selection) {
		kind, _ := el.Attr("type")
		if kind == "cite" || el.Find("blockquote").Len() > 0 || strings.TrimSpace(el.Text()) == "" {
			return
		}

		answered = true
	})

	return answered
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
		"hr#stopSpelling", "div.yahoo_quoted", "div.protonmail_quote", "blockquote.protonmail_quote",
		"div.moz-cite-prefix", `div[data-marker="__QUOTED_TEXT__"]`, `div[style*="border-top:solid #E1E1E1"]`,
		"#zwchr",
	}, ", ")
}

/*
HTMLPolicy returns the sanitizer policy for received HTML: the markup
mail is written in, a short list of inline styles, and links to the
web, mail, inline parts and embedded images only.
*/
func HTMLPolicy() *sanitize.Policy {
	p := sanitize.NewPolicy().
		AllowElements("a", "b", "i", "u", "s", "em", "strong", "p", "br", "div", "span", "blockquote", "ul",
			"ol", "li", "table", "thead", "tbody", "tfoot", "tr", "td", "th", "h1", "h2", "h3", "h4", "h5", "h6",
			"pre", "code", "hr", "img", "font", "center", "small", "sub", "sup", "strike", "del", "ins", "caption").
		AllowURLSchemes("http", "https", "mailto", "cid").
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
