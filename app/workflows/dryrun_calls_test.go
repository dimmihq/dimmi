package workflows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

func TestWaitWords(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2h", waitWords(workflowspec.WaitSettings{For: "2h"}))
	assert.Equal(t, "4h of office hours", waitWords(workflowspec.WaitSettings{For: "4h", Count: "business"}))
	assert.Equal(t, "until 09:00", waitWords(workflowspec.WaitSettings{Until: &workflowspec.WaitUntil{Time: "09:00"}}))
	assert.Equal(t, "until the office opens",
		waitWords(workflowspec.WaitSettings{Until: &workflowspec.WaitUntil{Kind: untilOpen}}))
}
