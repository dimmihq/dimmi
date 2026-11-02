/*
|--------------------------------------------------------------------------
| Text Cleanup
|--------------------------------------------------------------------------
|
| Text from a message is shown to people and stored, so it leaves here in
| one form: valid UTF-8, NFC normalized, without NUL bytes. Display names
| also lose bidirectional control characters, which a sender could use
| to make an address read as someone else's.
|
*/

package parse

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
Bounds on message id lists: how many ids are kept and how long one may
be, the longest line a header may hold.
*/
const (
	maxIDs      = 50
	maxIDLength = 998
)

/*
normalize cleans every text field and caps the warnings.
*/
func (p *Parsed) normalize(limits Limits) {
	p.Subject = clean(p.Subject)
	p.Text = clean(p.Text)
	p.HTML = clean(p.HTML)

	p.From = cleanAddress(p.From)
	for _, list := range [][]mail.Address{p.ReplyTo, p.To, p.Cc} {
		for i := range list {
			list[i] = cleanAddress(list[i])
		}
	}

	for i := range p.Attachments {
		p.Attachments[i].Filename = clean(p.Attachments[i].Filename)
	}

	if len(p.Warnings) > limits.MaxWarnings {
		p.Warnings = p.Warnings[:limits.MaxWarnings]
	}
}

func clean(s string) string {
	s = strings.ToValidUTF8(s, string(utf8.RuneError))
	s = strings.ReplaceAll(s, "\x00", "")

	return norm.NFC.String(s)
}

func cleanAddress(a mail.Address) mail.Address {
	a.Name = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Bidi_Control, r) {
			return -1
		}

		return r
	}, clean(a.Name))

	return a
}
