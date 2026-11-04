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
| A file name comes from the sender and ends up on disk listings and in
| download headers, so only its last path element survives, without
| control characters and without the trailing dots and spaces that
| Windows ignores: "invoice.exe." is an executable there.
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
		p.Attachments[i].Filename = cleanFilename(p.Attachments[i].Filename)
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

func cleanFilename(name string) string {
	name = clean(name)
	name = name[strings.LastIndexAny(name, `/\`)+1:]
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return -1
		}

		return r
	}, name)

	return strings.TrimRight(strings.TrimLeft(name, " "), ". ")
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
