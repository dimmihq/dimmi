package workflows

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

/*
responseValue reads the value at a plain dotted path in a JSON body,
like data.items.0.id: each part is an object key, or an index when it
walks into an array. Objects and arrays come back as their raw JSON
text and anything else as its decoded value; found is false when the
path leads nowhere.
*/
func responseValue(body []byte, path string) (value any, found bool) {
	if path == "" {
		return nil, false
	}

	raw := json.RawMessage(body)
	for part := range strings.SplitSeq(path, ".") {
		next, ok := child(raw, part)
		if !ok {
			return nil, false
		}

		raw = next
	}

	raw = bytes.TrimSpace(raw)
	if isContainer(raw) {
		return string(raw), true
	}

	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}

	return value, true
}

/*
child is the raw value under one part of a path: an object's key or
an array's index.
*/
func child(raw json.RawMessage, part string) (json.RawMessage, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, false
	}

	switch raw[0] {
	case '{':
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil {
			return nil, false
		}

		value, ok := object[part]

		return value, ok
	case '[':
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 {
			return nil, false
		}

		var array []json.RawMessage
		if json.Unmarshal(raw, &array) != nil || index >= len(array) {
			return nil, false
		}

		return array[index], true
	}

	return nil, false
}

/*
isContainer tells an object or an array from a single value.
*/
func isContainer(raw json.RawMessage) bool {
	return len(raw) > 0 && (raw[0] == '{' || raw[0] == '[')
}
