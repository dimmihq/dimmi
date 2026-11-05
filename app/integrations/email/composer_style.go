/*
|--------------------------------------------------------------------------
| Reply Styles
|--------------------------------------------------------------------------
|
| A reply looks plain by default: it reads as a person wrote it and
| lands best in the customer's inbox. An address may choose the branded
| style instead, which frames the same reply with the brand's color
| above it and a short footer below it. Both carry the same text twin,
| quote and signature; the footer joins the text twin too.
|
| The branded frame is built from tables with inline styles, the layout
| Outlook still renders, and it never loads anything from a server, so
| no reader is tracked and the mail shows the same offline.
|
*/

package email

import (
	"html"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
Reply styles an address may choose.
*/
const (
	StylePlain   = "plain"
	StyleBranded = "branded"
)

/*
defaultBrandColor frames a branded reply whose address chose no color.
*/
const defaultBrandColor = "#171717"

/*
brandColor is the shape of a color a branded reply may use.
*/
var brandColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

/*
Style is how a reply is framed. A zero Style is the plain look; a
branded one has its color, and its footer as HTML and as text when it
has one.
*/
type Style struct {
	Branded    bool
	Color      string
	FooterHTML string
	FooterText string
}

/*
RenderStyle returns the style of an address: its choice of look, its
brand color and its footer, written in the same small part of Markdown
as a signature. A color that is not a lowercase hex color falls back to
the default one.
*/
func RenderStyle(style, color, footer string) (Style, error) {
	if style != StyleBranded {
		return Style{}, nil
	}

	out := Style{Branded: true, Color: defaultBrandColor}
	if brandColor.MatchString(color) {
		out.Color = color
	}

	footer = strings.TrimSpace(strings.ReplaceAll(footer, "\r\n", "\n"))
	if footer == "" {
		return out, nil
	}

	policy := sanitize.NewPolicy().AllowElements("p", "br", "strong", "em").
		AllowURLSchemes("http", "https", "mailto", "tel").
		AllowAttrs("href").OnElements("a")

	rendered, err := markdown.New(markdown.Options{Sanitize: policy}).HTML([]byte(hardBreaks(footer)))
	if err != nil {
		return Style{}, err
	}

	out.FooterHTML, out.FooterText = rendered.HTML, footer

	return out, nil
}

/*
frame sets the body of a reply in its style.
*/
func frame(body string, s Style) string {
	if !s.Branded {
		return wrapHTML(body)
	}

	footer := ""
	if s.FooterHTML != "" {
		footer = `<tr><td style="padding:16px 24px;border-top:1px solid #e5e5e5;color:#737373;` +
			`font-size:13px;line-height:1.5">` + s.FooterHTML + `</td></tr>`
	}

	return `<!doctype html><html><body style="margin:0;background:#f5f5f5">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" ` +
		`style="background:#f5f5f5"><tr><td align="center" style="padding:24px 12px">` +
		`<table role="presentation" width="640" cellpadding="0" cellspacing="0" border="0" ` +
		`style="width:100%;max-width:640px;background:#ffffff;border-top:2px solid ` +
		html.EscapeString(s.Color) + `"><tr><td style="padding:24px;font-family:-apple-system,` +
		`BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;font-size:15px;line-height:1.5;` +
		`color:#171717">` + body + `</td></tr>` + footer + `</table></td></tr></table></body></html>`
}
