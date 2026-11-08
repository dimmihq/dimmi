/*
|--------------------------------------------------------------------------
| Agent Step Checks
|--------------------------------------------------------------------------
|
| The step that lets dimmi handle a conversation takes a few limits: how
| much extra guidance it adds, how many replies the agent may send, and
| how long a silent customer is waited for. Choices the engine cannot
| honor yet are refused at publish rather than silently ignored.
|
*/

package workflowspec

import (
	"time"
	"unicode/utf8"
)

/*
The bounds of the agent step's settings.
*/
const (
	maxDimmiInstructions = 1000
	maxDimmiReplies      = 50
	maxQuietAfter        = 7 * 24 * time.Hour
)

/*
checkDimmi checks the agent step's settings.
*/
func (c *checker) checkDimmi(s Step) {
	d, ok := decode[DimmiSettings](c, s)
	if !ok {
		return
	}

	if utf8.RuneCountInString(d.Instructions) > maxDimmiInstructions {
		c.fail(Issue{
			Code: CodeLimitExceeded, StepID: s.ID, Field: "instructions",
			Message: "Keep the instructions to 1,000 characters.",
		})
	}

	c.variables(s, "instructions", d.Instructions)
	if d.MaxReplies < 0 || d.MaxReplies > maxDimmiReplies {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: "max_replies", Message: "Allow 1 to 50 replies."})
	}

	c.duration(s, "quiet_after", d.QuietAfter, span{time.Minute, maxQuietAfter})
	c.dimmiNotYet(s, d)
}

/*
dimmiNotYet refuses the agent step's choices that cannot run yet.
*/
func (c *checker) dimmiNotYet(s Step, d DimmiSettings) {
	if d.HandoverNotice != "" && d.HandoverNotice != "dimmi" {
		c.fail(Issue{
			Code: CodeStepNotAvailable, StepID: s.ID, Field: "handover_notice",
			Message: "Letting the workflow announce the handover is not available yet.",
		})
	}

	if d.AskEmailBeforeHandover != nil && !*d.AskEmailBeforeHandover {
		c.fail(Issue{
			Code: CodeStepNotAvailable, StepID: s.ID, Field: "ask_email_before_handover",
			Message: "Turning off the email question before a handover is not available yet.",
		})
	}
}
