/*
|--------------------------------------------------------------------------
| Character Sets
|--------------------------------------------------------------------------
|
| Mail and old web pages still arrive in Windows-1252, ISO-8859-x, Shift
| JIS and the like, and their labels are often missing or wrong. The text
| is decoded to UTF-8 once, at the edge, the way a browser decides: a
| byte order mark first, then the declared charset, then a meta tag in
| the document, then UTF-8 if the bytes are valid UTF-8, and Windows-1252
| when nothing else fits.
|
*/

package dom

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/unicode"
)

/*
fallbackCharset is what the detector settles on when nothing names a
charset. It only looks at the first kilobyte, cut at a character
boundary, so a short text that is valid UTF-8 can still land here.
*/
const fallbackCharset = "windows-1252"

/*
DecodeCharset returns HTML bytes as a UTF-8 string. Declared is the
charset from the transport, such as a Content-Type header, or "" when
there is none; an unknown label counts as none.
*/
func DecodeCharset(b []byte, declared string) (string, error) {
	contentType := "text/html"
	if _, name := charset.Lookup(declared); name != "" {
		contentType += "; charset=" + name
	}

	enc, name, certain := charset.DetermineEncoding(b, contentType)
	if !certain && name == fallbackCharset && utf8.Valid(b) {
		enc = unicode.UTF8
	}

	out, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return "", fmt.Errorf("decode charset: %w", err)
	}

	return string(out), nil
}
