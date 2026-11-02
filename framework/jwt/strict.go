/*
|--------------------------------------------------------------------------
| Strict Token Parsing
|--------------------------------------------------------------------------
|
| Before any library sees a token, its three parts must be unpadded
| base64url and its header and payload JSON objects with no repeated key.
| A repeated key is the classic parser differential: one parser keeps the
| first "sub", another the last, and a token that says two things gets
| past whichever check reads the harmless one.
|
*/

package jwt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

/*
maxDepth bounds JSON nesting, so a hostile token cannot exhaust the
stack.
*/
const maxDepth = 32

/*
compact is a token split into its decoded parts.
*/
type compact struct {
	header  []byte
	payload []byte
}

/*
splitCompact trims surrounding whitespace, refuses a pasted "Bearer "
prefix and checks the size, then decodes and checks the three parts.
*/
func splitCompact(token string, maxSize int) (string, compact, error) {
	token = strings.TrimSpace(token)
	if len(token) > maxSize {
		return "", compact{}, ErrTooLarge
	}

	if len(token) > 7 && strings.EqualFold(token[:7], "bearer ") {
		return "", compact{}, fmt.Errorf("%w: remove the Bearer prefix", ErrMalformed)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", compact{}, fmt.Errorf("%w: expected three parts", ErrMalformed)
	}

	decoded := make([][]byte, len(parts))
	for i, part := range parts {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(part)
		if err != nil {
			return "", compact{}, fmt.Errorf("%w: use base64url without padding", ErrMalformed)
		}

		decoded[i] = raw
	}

	for _, object := range decoded[:2] {
		if err := checkObject(object); err != nil {
			return "", compact{}, err
		}
	}

	return token, compact{header: decoded[0], payload: decoded[1]}, nil
}

/*
checkObject returns ErrMalformed unless the data is exactly one JSON
object in which no object repeats a key.
*/
func checkObject(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return fmt.Errorf("%w: expected a json object", ErrMalformed)
	}

	if err := walkObject(dec, 1); err != nil {
		return err
	}

	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: data after the json object", ErrMalformed)
	}

	return nil
}

func walkValue(dec *json.Decoder, depth int) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: invalid json", ErrMalformed)
	}

	switch tok {
	case json.Delim('{'):
		return walkObject(dec, depth+1)
	case json.Delim('['):
		return walkArray(dec, depth+1)
	}

	return nil
}

func walkObject(dec *json.Decoder, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: json nested too deeply", ErrMalformed)
	}

	seen := map[string]bool{}

	for dec.More() {
		tok, err := dec.Token()
		key, ok := tok.(string)

		if err != nil || !ok {
			return fmt.Errorf("%w: invalid json", ErrMalformed)
		}

		if seen[key] {
			return fmt.Errorf("%w: duplicate key %q", ErrMalformed, key)
		}

		seen[key] = true

		if err := walkValue(dec, depth); err != nil {
			return err
		}
	}

	return closeDelim(dec)
}

func walkArray(dec *json.Decoder, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: json nested too deeply", ErrMalformed)
	}

	for dec.More() {
		if err := walkValue(dec, depth); err != nil {
			return err
		}
	}

	return closeDelim(dec)
}

func closeDelim(dec *json.Decoder) error {
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("%w: invalid json", ErrMalformed)
	}

	return nil
}
