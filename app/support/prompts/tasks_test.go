package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPrompts(t *testing.T) {
	t.Parallel()

	require.NoError(t, CheckTasks())
	names := TaskNames()
	require.NotEmpty(t, names)

	for _, name := range names {
		text := Task(name, nil)
		assert.NotEmpty(t, text, name)
		assert.NotContains(t, text, "<!--", "%s: the editors' note is never sent", name)
		if !strings.HasPrefix(name, "judgments/") {
			continue
		}
		parts := split(readTask(name))
		assert.NotEmpty(t, parts["Question"], name)
		assert.NotEmpty(t, parts["True when"], name)
		assert.NotEmpty(t, parts["False when"], name)
	}
}

func TestJudgmentParts(t *testing.T) {
	t.Parallel()

	parts := split("# State\nA reply.\n\n{{reply}}\n# Question\nDoes it ask?\n" +
		"# True when\nIt asks.\n# False when\nIt does not.")

	assert.Equal(t, "A reply.\n\n{{reply}}", parts["State"])
	assert.Equal(t, "Does it ask?", parts["Question"])
	assert.Equal(t, "It asks.", parts["True when"])
	assert.Equal(t, "It does not.", parts["False when"])
	assert.Equal(t, "A reply.\n\nHello", fill(parts["State"], Vars{"reply": "Hello"}))
	assert.Empty(t, Task("no/such/task", nil), "a task that does not exist is empty, which the tests catch")
}
