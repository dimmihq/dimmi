/*
|--------------------------------------------------------------------------
| Signed Cursors
|--------------------------------------------------------------------------
|
| A cursor carries the sort key and id of the last row a client saw, so
| the next page starts right after it and stays stable while rows are
| added. Clients treat it as opaque; the server signs it so nobody can
| hand-craft one to probe the database with values the API never gave
| out. A cursor that was changed, truncated or signed with another key is
| refused as invalid, never half-read.
|
*/

package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
Cursors signs and verifies cursors with one secret key.
*/
type Cursors struct {
	key []byte
}

/*
NewCursors returns cursors signed with the key, which must stay secret
and the same on every process serving the API.
*/
func NewCursors(key []byte) *Cursors {
	return &Cursors{key: bytes.Clone(key)}
}

/*
Encode returns the signed cursor for the value, typically a small struct
holding the last row's sort key and id. The value is stored as JSON, so
its exported fields are what survives the trip.
*/
func (c *Cursors) Encode(v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}

	enc := base64.RawURLEncoding

	return enc.EncodeToString(payload) + "." + enc.EncodeToString(hashing.HMAC(c.key, payload)), nil
}

/*
Decode verifies the cursor and reads it into the value. Any cursor that
was not made by Encode with this key, or does not fit the value, returns
ErrInvalidCursor.
*/
func (c *Cursors) Decode(s string, v any) error {
	payload, ok := c.verify(s)
	if !ok {
		return ErrInvalidCursor
	}

	if err := json.Unmarshal(payload, v); err != nil {
		return ErrInvalidCursor
	}

	return nil
}

/*
verify returns the payload of a cursor whose signature matches.
*/
func (c *Cursors) verify(s string) ([]byte, bool) {
	encodedPayload, encodedMAC, ok := strings.Cut(s, ".")
	if !ok {
		return nil, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, false
	}

	mac, err := base64.RawURLEncoding.DecodeString(encodedMAC)
	if err != nil {
		return nil, false
	}

	return payload, hashing.Equal(mac, hashing.HMAC(c.key, payload))
}
