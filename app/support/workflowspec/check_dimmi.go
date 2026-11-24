/*
|--------------------------------------------------------------------------
| Agent Step Checks
|--------------------------------------------------------------------------
|
| The step that hands a conversation to an AI agent takes few settings:
| which agent, whether it may transfer, and how long a silent customer
| is waited for. What the agent says comes from its own guidance and
| procedures, never from the step. A named agent must exist; the
| publish checks that.
|
*/

package workflowspec

import (
	"time"
)

/*
maxQuietAfter bounds how long the agent step waits on a silent customer.
*/
const maxQuietAfter = 7 * 24 * time.Hour

/*
checkDimmi checks the agent step's settings.
*/
func (c *checker) checkDimmi(s Step) {
	d, ok := decode[DimmiSettings](c, s)
	if !ok {
		return
	}

	c.duration(s, "quiet_after", d.QuietAfter, span{time.Minute, maxQuietAfter})
}
