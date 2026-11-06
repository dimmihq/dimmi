/*
|--------------------------------------------------------------------------
| AI Run Step Kind
|--------------------------------------------------------------------------
|
| A run is a trail of steps — classify, retrieve, generate, verify —
| each with its own provider, tokens and latency. The step kind is
| what makes one trace readable and one cost attributable.
|
*/

package enums

/*
AIRunStepKind names one stage of a run.
*/
type AIRunStepKind string

/*
The step kinds.
*/
const (
	AIRunStepKindClassify  AIRunStepKind = "classify"
	AIRunStepKindRetrieve  AIRunStepKind = "retrieve"
	AIRunStepKindGenerate  AIRunStepKind = "generate"
	AIRunStepKindToolCall  AIRunStepKind = "tool_call"
	AIRunStepKindVerify    AIRunStepKind = "verify"
	AIRunStepKindSummarize AIRunStepKind = "summarize"
	AIRunStepKindTranslate AIRunStepKind = "translate"
	AIRunStepKindProbe     AIRunStepKind = "probe"
)
