/*
|--------------------------------------------------------------------------
| Address Lists
|--------------------------------------------------------------------------
|
| An address field is first read by the rules. Real mail breaks them all
| the time, with an unquoted comma or colon in a name, a trailing dot on
| a domain, an empty address beside a good one, or no brackets at all. The
| address is still plainly there, and losing it would lose the sender, so
| a field that breaks the rules is scanned for whatever is an address and
| the text before it is taken as the name.
|
| The sender is what the first readable From field says, else the Sender
| field, else just a name. A sender read this way is never proven: mail
| authentication reads the header by the rules alone.
|
*/

package parse

import (
	"cmp"
	netmail "net/mail"
	"net/textproto"
	"strings"
	"unicode"

	"github.com/jhillyerd/enmime/v2"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
readSender returns who sent the message, with only a name when no field
holds an address.
*/
func readSender(fields textproto.MIMEHeader, message []byte) mail.Address {
	for _, value := range fields.Values("From") {
		if list := readAddresses(repairHeader(value, message)); len(list) > 0 {
			return list[0]
		}
	}

	name := ""
	for _, value := range fields.Values("From") {
		name = cmp.Or(name, plainName(repairHeader(value, message)))
	}

	for _, value := range fields.Values("Sender") {
		if list := readAddresses(repairHeader(value, message)); len(list) > 0 {
			list[0].Name = cmp.Or(name, list[0].Name)

			return list[0]
		}
	}

	return mail.Address{Name: name}
}

/*
readList returns the addresses in every occurrence of a field.
*/
func readList(fields textproto.MIMEHeader, field string, message []byte) []mail.Address {
	var out []mail.Address

	for _, value := range fields.Values(field) {
		out = append(out, readAddresses(repairHeader(value, message))...)
	}

	return out
}

/*
readAddresses reads one field value, by the rules when it can.
*/
func readAddresses(value string) []mail.Address {
	list, err := enmime.ParseAddressList(value)
	if err != nil {
		return scanAddresses(value)
	}

	out := make([]mail.Address, 0, len(list))
	for _, a := range list {
		out = append(out, mail.Address{Name: decodeWords(a.Name), Email: a.Address})
	}

	return out
}

/*
scanAddresses walks a field that breaks the rules. Text builds up as the
name of the next address, and an empty pair of brackets drops it, since
that name belonged to no address.
*/
func scanAddresses(value string) []mail.Address {
	var out []mail.Address

	nameStart := 0

	for position := 0; position < len(value); {
		token, next := nextToken(value, position)

		email := addressIn(token)
		switch {
		case email != "":
			out = append(out, mail.Address{Name: scannedName(value[nameStart:position]), Email: email})
			nameStart = next
		case strings.HasPrefix(token, "<"):
			nameStart = next
		}

		position = next
	}

	return out
}

/*
nextToken returns the token at a position and where the next one starts.
A token is a quoted string, a bracketed address or a run of anything
else; separators are tokens of their own, one character long.
*/
func nextToken(value string, position int) (string, int) {
	rest := value[position:]

	switch rest[0] {
	case '"':
		return closedToken(rest, position, '"', 1)
	case '<':
		return closedToken(rest, position, '>', len(rest))
	case ' ', '\t', ',', ';':
		return rest[:1], position + 1
	}

	end := strings.IndexAny(rest, " \t,;<\"")
	if end < 0 {
		end = len(rest)
	}

	return rest[:end], position + end
}

/*
closedToken returns a token up to its closing character, or the given
length of it when it is never closed.
*/
func closedToken(rest string, position int, closing byte, unclosed int) (string, int) {
	end := strings.IndexByte(rest[1:], closing)
	if end < 0 {
		return rest[:unclosed], position + unclosed
	}

	return rest[:end+2], position + end + 2
}

/*
addressIn returns the address a token holds, without what wraps it and
without a trailing dot on the domain, or "".
*/
func addressIn(token string) string {
	bare := strings.Trim(token, "<>\"'()[]:,;")
	bare = strings.TrimPrefix(strings.TrimSpace(bare), "mailto:")
	bare = strings.TrimRight(bare, ".")

	if !strings.Contains(bare, "@") {
		return ""
	}

	parsed, err := netmail.ParseAddress(bare)
	if err != nil || parsed.Name != "" {
		return ""
	}

	return parsed.Address
}

/*
scannedName tidies the text before an address into a name.
*/
func scannedName(text string) string {
	return decodeWords(strings.Trim(strings.ReplaceAll(text, `"`, ""), " \t,;:'"))
}

/*
plainName returns a field's text as a name when it says anything at all,
without a broken address that follows it.
*/
func plainName(value string) string {
	before, _, _ := strings.Cut(value, "<")
	name := decodeWords(strings.Trim(before, " \t,;:'\""))
	if !strings.ContainsFunc(name, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return ""
	}

	return name
}
