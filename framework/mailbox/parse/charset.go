/*
|--------------------------------------------------------------------------
| Character Sets
|--------------------------------------------------------------------------
|
| A declared charset is a claim, and mail often gets it wrong: Latin-1
| bytes labeled UTF-8, UTF-8 labeled Latin-1, labels no standard knows,
| Japanese escape sequences with no label at all. Bytes are therefore
| checked against the label, never trusted blindly. Whatever cannot be
| read as declared is read as UTF-8 where valid and Windows-1252 where
| not, so a stray byte costs one odd letter and never the whole message.
|
| The parser library is told to leave declared charsets alone, so the
| decision is taken here on the original bytes. It still guesses the
| charset of long text that declares none.
|
*/

package parse

import (
	"bytes"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jhillyerd/enmime/v2"
	"github.com/jhillyerd/enmime/v2/mediatype"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/dimmihq/dimmi/framework/dom"
)

/*
Labels this file decides on. Passthrough is a label the parser library
reads as UTF-8 without changing a byte.
*/
const (
	charsetPassthrough = "utf8mb4"
	charsetJapanese7   = "iso-2022-jp"
	charsetKorean7     = "iso-2022-kr"
	charsetUTF8        = "utf-8"
	charsetParam       = "charset"
)

/*
Escape sequences that open the two 7-bit charsets, which identify them
whatever the label says.
*/
const (
	escapeKorean     = "\x1b$)C"
	escapeJapaneseB  = "\x1b$B"
	escapeJapanese   = "\x1b$@"
	shiftOut         = 0x0e
	shiftIn          = 0x0f
	highBit          = 0x80
	asciiLimit       = 0x80
	headerForCharset = "Content-Type: text/plain; charset=\""
)

/*
canonicalCharset folds a declared label to one the decoders know, or ""
when it says nothing about the bytes. ASCII counts as saying nothing,
since mail that claims it and has high bytes is simply unlabeled.
*/
func canonicalCharset(label string) string {
	label = strings.ToLower(strings.Trim(label, " \t\"'"))
	label = strings.TrimPrefix(label, "charset=")
	label = strings.TrimPrefix(label, "x-windows-")

	switch {
	case strings.HasPrefix(label, "win-"):
		return "windows-" + strings.TrimPrefix(label, "win-")
	case isOnlyDigits(label) && strings.HasPrefix(label, "125"), label == "874":
		return "windows-" + label
	}

	return charsetAlias(label)
}

func isOnlyDigits(label string) bool {
	return label != "" && strings.Trim(label, "0123456789") == ""
}

/*
charsetAlias maps the labels found in real mail that no registry lists,
and widens the Chinese charsets to the one that contains them all, since
senders declare the narrow one and write the wide one.
*/
func charsetAlias(label string) string {
	switch label {
	case "", "unknown-8bit", "x-unknown", "unknown", "binary", "8bit", "7bit", "default", "x-user-defined",
		"us-ascii", "ascii", "ansi_x3.4-1968", "iso-646-us", "646", "us":
		return ""
	case "iso-gb2312", "gb2312", "gb_2312", "gb_2312-80", "gbk", "x-gbk", "cp936", "ms936", "windows-936", "euc-cn":
		return "gb18030"
	case "x-big5", "cp950", "ms950", "windows-950", "big5-hkscs":
		return "big5"
	case "ms932", "windows-932", "cp932", "x-ms-cp932", "sjis", "x-sjis":
		return "shift_jis"
	case "iso-2022-jp-1", "iso-2022-jp-2", "iso-2022-jp-3", "iso-2022-jp-ext", "csiso2022jp2":
		return charsetJapanese7
	case "csiso2022kr":
		return charsetKorean7
	case "unicode-1-1-utf-7", "x-utf-7", "utf7":
		return "utf-7"
	}

	return label
}

/*
mediaTypeReader reads a Content-Type or Content-Disposition for the
parser library, hiding every declared text charset from it. A part that
declares none is left for it to guess.
*/
func mediaTypeReader() enmime.CustomParseMediaType {
	return func(value string) (string, map[string]string, []string, error) {
		kind, params, invalid, err := mediatype.Parse(value)
		if err == nil && strings.HasPrefix(kind, "text/") {
			if canonicalCharset(params[charsetParam]) == "" {
				delete(params, charsetParam)
			} else {
				params[charsetParam] = charsetPassthrough
			}
		}

		return kind, params, invalid, err
	}
}

/*
decodedContent returns a text part's content as UTF-8.
*/
func decodedContent(part *enmime.Part) string {
	if part.Charset != "" && part.Charset != charsetPassthrough {
		return string(part.Content)
	}

	_, params, _, _ := mediatype.Parse(part.Header.Get("Content-Type"))

	content, _ := contentOf(part)

	return decodeBody(content, params[charsetParam], part.ContentType)
}

/*
decodeBody returns a body as UTF-8. HTML that declares nothing is also
searched for a charset named in the document itself.
*/
func decodeBody(raw []byte, label, mediaType string) string {
	if mediaType == typeHTML && canonicalCharset(label) == "" && !utf8.Valid(raw) {
		if out, err := dom.DecodeCharset(raw, ""); err == nil {
			return out
		}
	}

	return decodeText(raw, label)
}

/*
decodeText returns text as UTF-8, checking the bytes against the label.
*/
func decodeText(raw []byte, label string) string {
	if out, ok := decodeByMark(raw); ok {
		return out
	}

	label = labelFromEscapes(raw, canonicalCharset(label))

	switch label {
	case "", charsetUTF8, "utf8":
		return repairUTF8(raw)
	case charsetKorean7:
		return decodeKorean(raw)
	}

	enc, name := charset.Lookup(label)
	if enc == nil || name == "replacement" {
		return decodeByLibrary(raw, label)
	}

	if !multibyte(name) && isMultibyteUTF8(raw) {
		return string(raw)
	}

	out, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return repairUTF8(raw)
	}

	if bytes.ContainsRune(out, utf8.RuneError) && mostlyUTF8(raw) {
		return repairUTF8(raw)
	}

	return string(out)
}

/*
multibyte reports whether a charset has characters of several bytes, in
which text can look like UTF-8 by accident.
*/
func multibyte(name string) bool {
	names := []string{
		"big5", "euc-jp", "euc-kr", "gb18030", "gbk", "hz-gb-2312", "iso-2022-jp", "shift_jis", "utf-16be", "utf-16le",
	}

	return slices.Contains(names, name)
}

/*
mostlyUTF8 reports whether bytes that failed in their declared charset
hold more UTF-8 characters than bytes that are not UTF-8.
*/
func mostlyUTF8(raw []byte) bool {
	characters, strays := 0, 0

	for len(raw) > 0 {
		r, size := utf8.DecodeRune(raw)

		switch {
		case r == utf8.RuneError && size == 1:
			strays++
		case size > 1:
			characters++
		}

		raw = raw[size:]
	}

	return characters >= strays
}

/*
decodeByMark reads text that opens with a byte order mark, which outranks
any label.
*/
func decodeByMark(raw []byte) (string, bool) {
	marks := [][]byte{{0xef, 0xbb, 0xbf}, {0xff, 0xfe}, {0xfe, 0xff}}
	for _, mark := range marks {
		if !bytes.HasPrefix(raw, mark) {
			continue
		}

		out, _, err := transform.Bytes(unicode.BOMOverride(encoding.Nop.NewDecoder()), raw)
		if err != nil {
			return "", false
		}

		return string(out), true
	}

	return "", false
}

/*
labelFromEscapes returns the 7-bit charset whose escape sequences the
bytes hold, unless the label already names a real charset.
*/
func labelFromEscapes(raw []byte, label string) string {
	if label != "" && label != charsetUTF8 && label != "utf8" {
		return label
	}

	switch {
	case bytes.Contains(raw, []byte(escapeKorean)):
		return charsetKorean7
	case bytes.Contains(raw, []byte(escapeJapaneseB)), bytes.Contains(raw, []byte(escapeJapanese)):
		return charsetJapanese7
	}

	return label
}

/*
isMultibyteUTF8 reports whether bytes are valid UTF-8 and more than
ASCII, which real text in a single-byte charset almost never is.
*/
func isMultibyteUTF8(raw []byte) bool {
	return utf8.Valid(raw) && bytes.ContainsFunc(raw, func(r rune) bool { return r >= asciiLimit })
}

/*
repairUTF8 keeps what is valid UTF-8 and reads each other byte as
Windows-1252, the charset such bytes nearly always come from.
*/
func repairUTF8(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}

	var out strings.Builder

	for len(raw) > 0 {
		r, size := utf8.DecodeRune(raw)
		if r == utf8.RuneError && size == 1 {
			r = charmap.Windows1252.DecodeByte(raw[0])
		}

		out.WriteRune(r)
		raw = raw[size:]
	}

	return out.String()
}

/*
decodeKorean reads 7-bit Korean, which shifts into EUC-KR with its high
bits off. Text that already has them on is taken as it is.
*/
func decodeKorean(raw []byte) string {
	plain := bytes.ReplaceAll(raw, []byte(escapeKorean), nil)
	shifted := false

	out := make([]byte, 0, len(plain))
	for _, b := range plain {
		switch {
		case b == shiftOut:
			shifted = true
		case b == shiftIn:
			shifted = false
		case shifted && b > ' ':
			out = append(out, b|highBit)
		default:
			out = append(out, b)
		}
	}

	decoded, err := korean.EUCKR.NewDecoder().Bytes(out)
	if err != nil {
		return repairUTF8(raw)
	}

	return string(decoded)
}

/*
decodeByLibrary hands a label the web's charset table does not know to
the parser library, whose table is wider. Its decoder is not reachable
any other way than by reading a message.
*/
func decodeByLibrary(raw []byte, label string) string {
	if strings.ContainsAny(label, "\"\r\n;") {
		return repairUTF8(raw)
	}

	message := append([]byte(headerForCharset+label+"\"\r\n\r\n"), raw...)

	root, err := enmime.NewParser(enmime.DisableCharacterDetection(true)).ReadParts(bytes.NewReader(message))
	if err != nil || len(root.Errors) > 0 {
		return repairUTF8(raw)
	}

	return string(root.Content)
}
