/*
|--------------------------------------------------------------------------
| Uuencoded Parts
|--------------------------------------------------------------------------
|
| Old mail software still sends files uuencoded, a transfer encoding no
| MIME library handles. Stored as they arrive, such files are useless to
| anyone who opens them, so they are decoded here. Content that does not
| hold what the encoding promises is kept as it came.
|
*/

package parse

import (
	"bytes"
	"slices"
	"strings"

	"github.com/jhillyerd/enmime/v2"
)

/*
uuencodedName is the part of a transfer encoding's name that says the
content is uuencoded, as in x-uuencode, x-uue and uuencode.
*/
const uuencodedName = "uu"

/*
Layout of a uuencoded line: each character holds six bits above this
offset, and four of them make three bytes.
*/
const (
	uuOffset   = 32
	uuMask     = 0x3f
	uuGroup    = 4
	uuGroupOut = 3
)

/*
contentOf returns a part's content with any uuencoding undone, and the
file name the encoding carries, if any.
*/
func contentOf(part *enmime.Part) ([]byte, string) {
	encoding := strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Transfer-Encoding")))
	if !strings.HasPrefix(strings.TrimPrefix(encoding, "x-"), uuencodedName) {
		return part.Content, ""
	}

	decoded, name, ok := uudecode(part.Content)
	if !ok {
		return part.Content, ""
	}

	return decoded, name
}

/*
uudecode reads the lines between "begin" and "end". Some mailers send the
lines alone, so without a begin line every line has to look the part.
It reports whether the content was uuencoded at all.
*/
func uudecode(content []byte) ([]byte, string, bool) {
	lines := bytes.Split(content, []byte("\n"))
	name, body, started := afterBegin(lines)

	if !started && slices.ContainsFunc(lines, isNotData) {
		return nil, "", false
	}

	var out []byte

	for _, line := range body {
		line = bytes.TrimRight(line, "\r")
		if string(bytes.TrimSpace(line)) == "end" {
			break
		}

		out = append(out, uudecodeLine(line)...)
	}

	return out, name, true
}

/*
afterBegin returns the name on the begin line and the lines after it, or
all the lines when there is none.
*/
func afterBegin(lines [][]byte) (string, [][]byte, bool) {
	for i, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) >= 3 && fields[0] == "begin" {
			return strings.Join(fields[2:], " "), lines[i+1:], true
		}
	}

	return "", lines, false
}

/*
isNotData reports whether a line, without a begin line to vouch for it,
cannot be uuencoded: it holds fewer characters than its first says.
*/
func isNotData(line []byte) bool {
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 {
		return false
	}

	return (len(line)-1)*uuGroupOut/uuGroup < int((line[0]-uuOffset)&uuMask)
}

/*
uudecodeLine decodes one line, whose first character says how many bytes
it holds. Trailing spaces some mailers strip count as zero bits.
*/
func uudecodeLine(line []byte) []byte {
	if len(line) == 0 {
		return nil
	}

	want := int((line[0] - uuOffset) & uuMask)
	out := make([]byte, 0, want+uuGroupOut)

	sextet := func(i int) byte {
		if i >= len(line) {
			return 0
		}

		return (line[i] - uuOffset) & uuMask
	}

	for i := 1; len(out) < want; i += uuGroup {
		a, b, c, d := sextet(i), sextet(i+1), sextet(i+2), sextet(i+3)
		out = append(out, a<<2|b>>4, b<<4|c>>2, c<<6|d)
	}

	return out[:want]
}
