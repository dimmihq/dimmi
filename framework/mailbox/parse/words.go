/*
|--------------------------------------------------------------------------
| Encoded Words
|--------------------------------------------------------------------------
|
| Header text with non-ASCII characters travels as encoded words, and
| real clients get them wrong: excess padding, a multi-byte character or
| even a base64 group split across two words, labels nobody registered,
| or no encoding at all, just raw bytes in the header. Each of these
| still names a person or a subject someone will read, so they are read
| as far as the bytes allow instead of being shown raw.
|
*/

package parse

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strings"
)

var encodedWord = regexp.MustCompile(`=\?([^?\s]+)\?([bBqQ])\?(.*?)\?=`)

/*
wordRun collects adjacent encoded words in one charset. Their bytes are
decoded together, since a character can be split between words. Pending
holds base64 still waiting for the rest of its group.
*/
type wordRun struct {
	charset string
	data    []byte
	pending string
}

/*
decodeWords replaces the encoded words in header text. White space
between two encoded words is not part of the text.
*/
func decodeWords(text string) string {
	matches := encodedWord.FindAllStringSubmatchIndex(text, -1)
	if matches == nil {
		return text
	}

	var (
		out strings.Builder
		run wordRun
	)

	end := 0

	for _, m := range matches {
		between := text[end:m[0]]
		label := strings.ToLower(strings.SplitN(text[m[2]:m[3]], "*", 2)[0])

		if strings.TrimSpace(between) != "" || end == 0 {
			out.WriteString(run.flush())
			out.WriteString(between)
		} else if run.charset != label {
			out.WriteString(run.flush())
		}

		run.charset = label
		run.add(text[m[4]:m[5]], text[m[6]:m[7]])
		end = m[1]
	}

	out.WriteString(run.flush())
	out.WriteString(text[end:])

	return out.String()
}

/*
add takes one word's payload. A word that ends a base64 group, with
padding or an exact length, is complete; one that does not waits for the
next word.
*/
func (r *wordRun) add(encoding, payload string) {
	if strings.EqualFold(encoding, "Q") {
		r.drain()
		r.data = append(r.data, decodeQ(payload)...)

		return
	}

	bare := strings.TrimRight(payload, "=")
	r.pending += bare

	if len(bare) != len(payload) || len(r.pending)%4 == 0 {
		r.drain()
	}
}

/*
drain decodes the base64 waiting, leaving out what cannot be base64 and
a last character that cannot complete a byte.
*/
func (r *wordRun) drain() {
	clean := strings.Map(func(c rune) rune {
		if strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", c) {
			return c
		}

		return -1
	}, r.pending)
	r.pending = ""

	if len(clean)%4 == 1 {
		clean = clean[:len(clean)-1]
	}

	decoded, _ := base64.RawStdEncoding.DecodeString(clean)
	r.data = append(r.data, decoded...)
}

/*
flush returns the text of the words collected and starts a new run.
*/
func (r *wordRun) flush() string {
	r.drain()

	text := decodeText(r.data, r.charset)
	*r = wordRun{}

	return text
}

/*
decodeQ reads the Q encoding, keeping an equals sign that does not start
a hex pair.
*/
func decodeQ(payload string) []byte {
	out := make([]byte, 0, len(payload))

	for i := 0; i < len(payload); i++ {
		switch payload[i] {
		case '_':
			out = append(out, ' ')
		case '=':
			if b, ok := hexPair(payload[i+1:]); ok {
				out = append(out, b)
				i += 2

				continue
			}

			out = append(out, '=')
		default:
			out = append(out, payload[i])
		}
	}

	return out
}

func hexPair(text string) (byte, bool) {
	if len(text) < 2 {
		return 0, false
	}

	b, err := hex.DecodeString(text[:2])
	if err != nil {
		return 0, false
	}

	return b[0], true
}
