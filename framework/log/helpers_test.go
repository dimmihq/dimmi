package log_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/log"
)

/*
newJSON returns a debug-level JSON logger writing into the returned
buffer.
*/
func newJSON(t *testing.T) (log.Logger, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}

	logger, err := log.New(log.Options{Driver: "json", Level: "debug", Writer: out})
	require.NoError(t, err)

	return logger, out
}

/*
lines decodes every JSON line written to the buffer.
*/
func lines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()

	var decoded []map[string]any

	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	for decoder.More() {
		var line map[string]any
		require.NoError(t, decoder.Decode(&line))
		decoded = append(decoded, line)
	}

	return decoded
}

/*
onlyLine decodes the buffer, requires exactly one line and returns it.
*/
func onlyLine(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()

	decoded := lines(t, out)
	require.Len(t, decoded, 1)

	return decoded[0]
}
