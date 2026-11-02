/*
|--------------------------------------------------------------------------
| Ciphertext Header
|--------------------------------------------------------------------------
|
| A ciphertext starts with a format version and the scope whose keys
| sealed it, so decryption reads the scope instead of guessing it. The
| header is authenticated along with the data: changing the scope or the
| version makes decryption fail.
|
*/

package encryption

import (
	"encoding/binary"
	"fmt"
)

/*
formatVersion is the first byte of every ciphertext; maxField bounds a
header string, so a corrupt length cannot make decoding allocate much.
*/
const (
	formatVersion = byte(1)
	maxField      = 255
)

/*
encodeHeader returns the version followed by the scope.
*/
func encodeHeader(scope Scope) []byte {
	return append([]byte{formatVersion}, encodeScope(scope)...)
}

func encodeScope(scope Scope) []byte {
	return appendString(appendString(nil, scope.Kind), scope.ID)
}

func appendString(b []byte, s string) []byte {
	return append(binary.AppendUvarint(b, uint64(len(s))), s...)
}

/*
header is a decoded ciphertext header: the scope, the header bytes and
the Tink ciphertext after them.
*/
type header struct {
	scope Scope
	raw   []byte
	body  []byte
}

func decodeHeader(ciphertext []byte) (header, error) {
	if len(ciphertext) == 0 || ciphertext[0] != formatVersion {
		return header{}, fmt.Errorf("%w: unknown format", ErrMalformed)
	}

	kind, rest, err := readString(ciphertext[1:])
	if err != nil {
		return header{}, err
	}

	id, rest, err := readString(rest)
	if err != nil {
		return header{}, err
	}

	raw := ciphertext[:len(ciphertext)-len(rest)]

	return header{scope: Scope{Kind: kind, ID: id}, raw: raw, body: rest}, nil
}

func readString(b []byte) (string, []byte, error) {
	n, read := binary.Uvarint(b)
	if read <= 0 || n > maxField {
		return "", nil, fmt.Errorf("%w: bad header", ErrMalformed)
	}

	end := read + int(n)
	if end > len(b) {
		return "", nil, fmt.Errorf("%w: bad header", ErrMalformed)
	}

	return string(b[read:end]), b[end:], nil
}
