/*
|--------------------------------------------------------------------------
| Reply Signatures
|--------------------------------------------------------------------------
|
| A signature closes a reply below its text and above the quote. It is
| written in a small part of Markdown, bold, italic, links and line
| breaks, so it reads the same in every client and can never carry a
| script, an image from a server or anything else that tracks the
| reader. Placeholders name the teammate, the address and the company,
| and are filled in when the reply is written; a line whose placeholder
| has nothing to show is left out rather than sent empty.
|
| The text part starts the signature with the "-- " line, so mail
| clients, and our own reader of replies, know where it begins.
|
*/

package email

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
signatureDelimiter opens the signature in the text part.
*/
const signatureDelimiter = "-- "

/*
signaturePlaceholder finds the placeholders of a signature.
*/
var signaturePlaceholder = regexp.MustCompile(`\{\{\s*([^{}]*?)\s*\}\}`)

/*
Signature is a signature ready to put under a reply: its HTML and its
text, the latter starting with the "-- " line.
*/
type Signature struct {
	HTML string
	Text string
}

/*
SignatureValues fill the placeholders of a signature: the teammate's
name and title, the address's display name and the company's name.
*/
type SignatureValues struct {
	MemberName  string
	MemberTitle string
	AddressName string
	CompanyName string
}

/*
SignaturePlaceholders returns the placeholders a signature may use.
*/
func SignaturePlaceholders() []string {
	return []string{"member_name", "member_title", "address_name", "company_name"}
}

/*
UnknownSignaturePlaceholders returns the placeholders of the body that
are not among the ones a signature may use, each once, in order.
*/
func UnknownSignaturePlaceholders(body string) []string {
	var unknown []string

	for _, match := range signaturePlaceholder.FindAllStringSubmatch(body, -1) {
		if !slices.Contains(SignaturePlaceholders(), match[1]) && !slices.Contains(unknown, match[0]) {
			unknown = append(unknown, match[0])
		}
	}

	return unknown
}

/*
RenderSignature fills in the body's placeholders and returns the
signature, empty when nothing is left to show. Unknown placeholders
stay as they were written.
*/
func RenderSignature(body string, values SignatureValues) (Signature, error) {
	source := fill(body, values, escapeMarkdown)
	if strings.TrimSpace(source) == "" {
		return Signature{}, nil
	}

	policy := sanitize.NewPolicy().AllowElements("p", "br", "strong", "em").
		AllowURLSchemes("http", "https", "mailto", "tel").
		AllowAttrs("href").OnElements("a")

	rendered, err := markdown.New(markdown.Options{Sanitize: policy}).HTML([]byte(hardBreaks(source)))
	if err != nil {
		return Signature{}, err
	}

	return Signature{
		HTML: `<div style="margin-top:16px;color:#525252">` + rendered.HTML + `</div>`,
		Text: signatureDelimiter + "\n" + strings.TrimSpace(fill(body, values, strings.TrimSpace)),
	}, nil
}

/*
fill replaces the known placeholders line by line, each value passed
through shape first. A line that had a placeholder and has no letter or
digit left once filled is dropped.
*/
func fill(body string, values SignatureValues, shape func(string) string) string {
	lookup := map[string]string{
		"member_name": values.MemberName, "member_title": values.MemberTitle,
		"address_name": values.AddressName, "company_name": values.CompanyName,
	}

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		filled := signaturePlaceholder.ReplaceAllStringFunc(line, func(token string) string {
			name := signaturePlaceholder.FindStringSubmatch(token)[1]
			if value, ok := lookup[name]; ok {
				return shape(value)
			}

			return token
		})

		if filled != line && !strings.ContainsFunc(filled, isWordRune) {
			continue
		}

		out = append(out, filled)
	}

	return strings.Join(out, "\n")
}

/*
hardBreaks keeps every line break of the signature as a break.
*/
func hardBreaks(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = strings.TrimRight(line, " ") + "  "
		}
	}

	return strings.Join(lines, "\n")
}

/*
escapeMarkdown keeps a filled-in value from reading as Markdown.
*/
func escapeMarkdown(value string) string {
	var out strings.Builder

	for _, r := range strings.TrimSpace(value) {
		if strings.ContainsRune("\\`*_{}[]()<>#+-.!|~", r) {
			out.WriteRune('\\')
		}

		out.WriteRune(r)
	}

	return out.String()
}

/*
isWordRune reports a letter or digit.
*/
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
