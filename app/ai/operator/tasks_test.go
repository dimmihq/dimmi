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
placeholder unfilled, the questions Operator is asked are the files'
text, and every skill has a file and a line in the list of skills.
*/
func TestTaskFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		vars []string
	}{
		{name: taskSystem},
		{name: taskSession, vars: []string{"workspace", "teammate", "role", "today"}},
		{name: taskRoutine, vars: []string{"workspace", "today", "name", "suggesting", "window", "skills"}},
		{name: taskSubagent, vars: []string{"workspace", "today", "name", "window", "skills"}},
		{name: taskRoutineSuggesting},
		{name: taskRoutineWindow, vars: []string{"start", "end", "start_date", "end_date", "last_run"}},
		{name: taskRoutineFirstRun},
		{name: taskRoutineLastRun, vars: []string{"at"}},
		{name: taskSkills, vars: []string{"list"}},
		{name: taskWriteReminder},
		{name: taskReplyDrafting},
		{name: taskLastCall},
		{name: taskThreadConversation, vars: []string{"number"}},
		{name: taskInstructions, vars: []string{"instructions"}},
		{name: taskMemories, vars: []string{"memories"}},
		{name: taskBrief},
		{name: taskTriage},
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

	for _, name := range skillNames() {
		assert.NotEmpty(t, skillBody(name), "the skill %s has a file", name)
		assert.Contains(t, skillList(), "- "+name+": when ", "the list of skills offers %s", name)
	}
}
