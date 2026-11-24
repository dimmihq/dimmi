package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
TestTaskFiles holds Operator to its task files: each exists, puts
every value the code passes where the model will read it and leaves no
placeholder unfilled, and the questions Operator is asked are the
files' text.
*/
func TestTaskFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		vars []string
	}{
		{name: taskSystem, vars: []string{"workspace", "teammate", "role", "today"}},
		{name: taskThreadConversation, vars: []string{"number"}},
		{name: taskInstructions, vars: []string{"instructions"}},
		{name: taskMemories, vars: []string{"memories"}},
		{name: taskBrief},
		{name: taskTriage},
		{name: taskProcedureReview},
	}
	for _, c := range cases {
		require.NotEmpty(t, prompts.Task(c.name, nil), c.name)

		vars := prompts.Vars{}
		for _, v := range c.vars {
			vars[v] = "value of " + v
		}

		text := prompts.Task(c.name, vars)
		for _, v := range c.vars {
			assert.Contains(t, text, "value of "+v, "%s reads {{%s}}", c.name, v)
		}

		assert.NotContains(t, text, "{{", "%s has a placeholder the code does not fill", c.name)
	}

	assert.Equal(t, prompts.Task(taskBrief, nil), BriefQuestion())
	assert.NotEmpty(t, TriageQuestion())
	assert.NotEmpty(t, ProcedureQAQuestion())
}
