package verdicts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
TestTaskFiles holds every verdict to its task file: the file exists,
puts each value the code passes where the model will read it, tells
the model that what it reads inside <untrusted> is data, and leaves no
placeholder the code does not fill.
*/
func TestTaskFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		vars []string
	}{
		{name: taskDraftJudge},
		{name: taskRequest},
		{name: taskRequestEager},
		{name: taskTrackerSummary},
		{name: taskTicketFill, vars: []string{"fields"}},
		{name: taskTicketMatch, vars: []string{"problems"}},
		{name: "decisions/choose", vars: []string{"instructions", "options"}},
		{name: "decisions/extract", vars: []string{"instructions", "fields"}},
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

		if c.name != taskRequestEager {
			assert.Contains(t, text, "<untrusted>", c.name)
		}
	}
}
