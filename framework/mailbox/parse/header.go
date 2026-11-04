/*
|--------------------------------------------------------------------------
| Message Header
|--------------------------------------------------------------------------
|
| Who wrote a message and what about matters more than anything that can
| go wrong below it, so the header is read on its own, before and apart
| from the MIME structure. Junk ahead of the first field and stray lines
| among the fields, which a strict reader would stop at or glue onto the
| field before them, are skipped.
|
| Text that has no recognizable header at all is not mail, and is left
| for the caller as plain text.
|
*/

package parse

import (
	"bytes"
	"net/textproto"
	"regexp"
	"strings"
	"unicode/utf8"
)

/*
Bounds on reading a header: how many lines of junk may precede it, and
how much of a message is searched for a charset its raw header bytes may
be in.
*/
const (
	maxJunkLines      = 5
	maxCharsetSearch  = 1 << 20
	shiftJISFor8BitOf = "shift_jis"
)

var charsetDeclaration = regexp.MustCompile(`(?i)charset\s*=\s*"?([a-z0-9_.:-]+)`)

/*
head is the header of a message, with the raw message from its first
field on and the body that follows the header.
*/
type head struct {
	fields  textproto.MIMEHeader
	message []byte
	body    []byte
}

/*
readHead finds and reads the header of raw text, and reports whether it
has one.
*/
func readHead(raw []byte) (head, bool) {
	start, found := fieldsStart(raw)
	if !found {
		return head{}, false
	}

	message := raw[start:]
	fields, body := readFields(message)

	if !isMailHeader(fields) {
		return head{}, false
	}

	return head{fields: fields, message: message, body: body}, true
}

/*
fieldsStart returns where the first header field starts, skipping a few
lines that are not fields, such as blank lines, an mbox separator or a
line a forwarding tool put on top.
*/
func fieldsStart(raw []byte) (int, bool) {
	position := 0

	for range maxJunkLines {
		if position >= len(raw) {
			break
		}

		line, next := lineAt(raw, position)
		if isField(line) {
			return position, true
		}

		position = next
	}

	return 0, false
}

/*
readFields reads header fields up to the blank line that ends them,
leaving out lines that are not fields, and returns what follows.
*/
func readFields(message []byte) (textproto.MIMEHeader, []byte) {
	fields := textproto.MIMEHeader{}
	position, name, value := 0, "", ""

	flush := func() {
		if name != "" {
			fields.Add(name, strings.TrimSpace(value))
		}
	}

	for position < len(message) {
		line, next := lineAt(message, position)
		position = next

		switch {
		case len(line) == 0:
			flush()

			return fields, message[position:]
		case line[0] == ' ' || line[0] == '\t':
			value = strings.TrimRight(value, " \t") + " " + strings.TrimSpace(string(line))
		case isField(line):
			flush()

			key, rest, _ := strings.Cut(string(line), ":")
			name, value = key, rest
		}
	}

	flush()

	return fields, nil
}

/*
lineAt returns the line at a position, without its ending, and where the
next one starts.
*/
func lineAt(raw []byte, position int) ([]byte, int) {
	rest := raw[position:]

	end := bytes.IndexByte(rest, '\n')
	if end < 0 {
		return bytes.TrimRight(rest, "\r"), len(raw)
	}

	return bytes.TrimRight(rest[:end], "\r"), position + end + 1
}

/*
isField reports whether a line starts a header field: a name of printable
characters without spaces, then a colon.
*/
func isField(line []byte) bool {
	colon := bytes.IndexByte(line, ':')
	if colon <= 0 {
		return false
	}

	for _, c := range line[:colon] {
		if c <= ' ' || c > '~' {
			return false
		}
	}

	return true
}

/*
isMailHeader reports whether fields include one that every kind of mail
software writes, which text that merely has a colon in it does not.
*/
func isMailHeader(fields textproto.MIMEHeader) bool {
	for _, name := range []string{
		"From", "Sender", "To", "Cc", "Subject", "Date", "Received", "Return-Path", "Message-Id", "Reply-To",
		"Mime-Version", "Content-Type", "Delivered-To",
	} {
		if fields.Get(name) != "" {
			return true
		}
	}

	return false
}

/*
repairHeader makes raw header bytes readable. A sender that does not
encode its header usually writes it in the charset of the body, and in
Japanese mail in the 8-bit form of it.
*/
func repairHeader(value string, message []byte) string {
	if utf8.ValidString(value) {
		return value
	}

	return decodeText([]byte(value), rawHeaderCharset(message))
}

/*
rawHeaderCharset returns the first charset a message declares that could
explain raw bytes in its header, or "".
*/
func rawHeaderCharset(message []byte) string {
	search := message[:min(len(message), maxCharsetSearch)]

	for _, match := range charsetDeclaration.FindAllSubmatch(search, -1) {
		label := canonicalCharset(string(match[1]))

		switch label {
		case "", charsetUTF8, "utf8":
			continue
		case charsetJapanese7:
			return shiftJISFor8BitOf
		}

		return label
	}

	return ""
}
