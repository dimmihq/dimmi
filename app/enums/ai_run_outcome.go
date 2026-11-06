/*
|--------------------------------------------------------------------------
| AI Run Outcome
|--------------------------------------------------------------------------
|
| What a run decided is reported honestly: a real answer is not the
| same as an "I don't know", and silence is never counted as success.
|
*/

package enums

/*
AIRunOutcome names how one run ended.
*/
type AIRunOutcome string

/*
The run outcomes.
*/
const (
	AIRunOutcomeAnswered   AIRunOutcome = "answered"
	AIRunOutcomeUnknown    AIRunOutcome = "unknown"
	AIRunOutcomeHandoff    AIRunOutcome = "handoff"
	AIRunOutcomeSilent     AIRunOutcome = "silent"
	AIRunOutcomeGreeted    AIRunOutcome = "greeted"
	AIRunOutcomeRefused    AIRunOutcome = "refused"
	AIRunOutcomeSkipped    AIRunOutcome = "skipped"
	AIRunOutcomeSuperseded AIRunOutcome = "superseded"
	AIRunOutcomeError      AIRunOutcome = "error"
)
