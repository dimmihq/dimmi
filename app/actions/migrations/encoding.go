/*
|--------------------------------------------------------------------------
| Record Encoding
|--------------------------------------------------------------------------
|
| Hashing records so a resend in another key order is still the same,
| and the small JSON helpers the ledger uses.
|
*/

package migrations

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
problemOf returns the refusal as a record's problem, a generic one for
an error that is not a refusal.
*/
func problemOf(err error) *Problem {
	if f, ok := errors.AsType[*fault.Error](err); ok {
		return &Problem{Code: f.Code(), Message: f.Message(), Field: f.Field()}
	}

	return &Problem{Code: "migration_record_error", Message: "dimmi could not apply this record. Send it again."}
}

/*
contentHash is the hash of the record's JSON with its keys sorted, so
the same record sent with its keys in another order is still the same.
*/
func contentHash(raw json.RawMessage) string {
	return hashing.SHA256Hex(canonical(raw))
}

/*
canonical returns the JSON with its object keys sorted.
*/
func canonical(raw json.RawMessage) []byte {
	var v any

	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()

	if err := d.Decode(&v); err != nil {
		return raw
	}

	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}

	return out
}

/*
compactJSON returns the JSON compacted, or a JSON string of it when it
is not JSON.
*/
func compactJSON(raw json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return mustJSON(string(raw))
	}

	return buf.Bytes()
}

/*
mustJSON encodes the value, which is always encodable.
*/
func mustJSON(v any) json.RawMessage {
	out, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}

	return out
}

/*
nonNil returns the list, empty instead of nil.
*/
func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}

/*
nonNilRefs returns the references, empty instead of nil.
*/
func nonNilRefs(list []Ref) []Ref {
	return nonNil(list)
}
