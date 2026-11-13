/*
|--------------------------------------------------------------------------
| Scenarios
|--------------------------------------------------------------------------
|
| A scenario is one simulated customer: who they are, what they want,
| and, for the scripted simulator, the messages they send in order. Its
| kind names the shape of conversation it stands for, because cost
| depends on the shape: one quick answer, several searches, a long
| troubleshooting thread, action calls, a customer who wants a person,
| a question the help center does not cover, another language, or a
| wall of pasted logs.
|
*/

package costsim

/*
The kinds of scenario, by the shape of conversation they stand for.
*/
const (
	KindOneAnswer       = "one_answer"
	KindMultiSearch     = "multi_search"
	KindTroubleshooting = "troubleshooting"
	KindActions         = "actions"
	KindVague           = "vague"
	KindWantsHuman      = "wants_human"
	KindUncovered       = "uncovered"
	KindNonEnglish      = "non_english"
	KindLongLogs        = "long_logs"
)

/*
Kinds lists every kind, in report order.
*/
func Kinds() []string {
	return []string{
		KindOneAnswer, KindMultiSearch, KindTroubleshooting, KindActions, KindVague, KindWantsHuman,
		KindUncovered, KindNonEnglish, KindLongLogs,
	}
}

/*
Scenario is one simulated customer. Signed marks a customer who is
signed in to the product, so actions bound to their account can run.
Messages are the scripted customer's turns, the first one opening the
conversation; the model-played customer opens with the first one too,
then writes the rest itself from the persona and the goal.
*/
type Scenario struct {
	Key      string
	Kind     string
	Persona  string
	Goal     string
	Locale   string
	Signed   bool
	Subject  string
	Messages []string
}
