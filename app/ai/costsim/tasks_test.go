package costsim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
TestTaskFiles holds the model-played customer to its task files: each
exists, the customer's prompt puts every value the code passes where
the model will read it, and no placeholder is left unfilled.
*/
func TestTaskFiles(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, prompts.Task(taskCustomerWaiting, nil))
	require.NotEmpty(t, prompts.Task(taskCustomer, nil))

	vars := prompts.Vars{}
	for _, v := range []string{"company", "persona", "goal", "locale"} {
		vars[v] = "value of " + v
	}

	vars["done_marker"] = DoneMarker

	text := prompts.Task(taskCustomer, vars)
	for name, value := range vars {
		assert.Contains(t, text, value, "the customer's prompt reads {{%s}}", name)
	}

	assert.NotContains(t, text, "{{", "the customer's prompt has a placeholder the code does not fill")
}
