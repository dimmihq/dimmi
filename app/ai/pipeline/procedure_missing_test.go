package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMissing(t *testing.T) {
	t.Parallel()

	read := func(raw string) missing {
		var in struct {
			Missing missing `json:"still_missing"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &in), raw)
		return in.Missing
	}

	assert.Equal(t, missing{}, read(`{"still_missing":[]}`), "an empty list is a step with nothing missing")
	assert.Equal(t, missing{}, read(`{"still_missing":["", "  "]}`), "blank items are not things missing")
	assert.Equal(t, missing{}, read(`{"still_missing":""}`), "an empty text is read as an empty list")
	assert.Equal(t, missing{Items: []string{"the order number", "the email"}},
		read(`{"still_missing":["the order number"," the email "]}`))
	assert.Equal(t, missing{Sentence: true}, read(`{"still_missing":"Nothing."}`),
		"a sentence is sent back, never read for what it means")

	note, err := listNote()
	require.NoError(t, err)
	assert.Contains(t, note, `"ok":false`)
	assert.Contains(t, note, "must be a list")

	schema := string(procedureStatusSpec().Schema)
	assert.Contains(t, schema, `"still_missing":{"description"`)
	assert.Contains(t, schema, `"items":{"type":"string"},"type":"array"`,
		"what is missing is a list, so nothing missing is an empty list and never a word")
}
