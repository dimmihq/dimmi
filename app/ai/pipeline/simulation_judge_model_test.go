package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/prompts"
)

func TestReadVerdicts(t *testing.T) {
	t.Parallel()

	t.Run("reads one verdict per check, in the checks' order, with the judge's reason", func(t *testing.T) {
		t.Parallel()
		out := readVerdicts(`{"verdicts":[
			{"requirement":2,"reason":" Rio never names a price for Enterprise. ","met":true},
			{"requirement":1,"reason":"Rio lists the plans before asking how much the shop ships.","met":false}]}`, 2)

		require.Len(t, out, 2)
		assert.False(t, out[0].Passed)
		assert.Equal(t, "Rio lists the plans before asking how much the shop ships.", out[0].Message)
		assert.True(t, out[1].Passed)
		assert.Equal(t, "Rio never names a price for Enterprise.", out[1].Message)
		assert.False(t, out[0].Unsure, "a judge that writes says met or not met")
	})

	t.Run("says met or not met when the judge gave no reason, and cuts a long one", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("a", reasonRunes+50)
		out := readVerdicts(`{"verdicts":[{"requirement":1,"reason":"","met":true},`+
			`{"requirement":2,"reason":"  ","met":false},{"requirement":3,"reason":"`+long+`","met":true}]}`, 3)

		require.Len(t, out, 3)
		assert.Equal(t, "Met.", out[0].Message)
		assert.Equal(t, "Not met.", out[1].Message)
		assert.LessOrEqual(t, len([]rune(out[2].Message)), reasonRunes+1)
	})

	t.Run("reads nothing unless every check has exactly one verdict", func(t *testing.T) {
		t.Parallel()
		two := func(first, second int) string {
			return fmt.Sprintf(`{"verdicts":[{"requirement":%d,"reason":"x","met":true},`+
				`{"requirement":%d,"reason":"y","met":false}]}`, first, second)
		}
		for name, text := range map[string]string{
			"not JSON":        `met`,
			"one too few":     `{"verdicts":[{"requirement":1,"reason":"x","met":true}]}`,
			"the same twice":  two(1, 1),
			"a number beyond": two(1, 3),
			"a number below":  two(0, 2),
		} {
			assert.Nil(t, readVerdicts(text, 2), name)
		}
	})
}

func TestJudgePrompt(t *testing.T) {
	t.Parallel()

	text := prompts.Task("simulation/judge", nil)
	require.NotEmpty(t, text)
	assert.NotContains(t, text, "<!--", "the editors' note is not sent")
	assert.NotContains(t, text, "{{", "it has no placeholder left to fill")
	assert.JSONEq(t, verdictSchema, verdictSchema, "the schema is valid JSON")
}
