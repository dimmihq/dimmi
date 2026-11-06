/*
|--------------------------------------------------------------------------
| Decider Contract
|--------------------------------------------------------------------------
|
| Routing a customer's message — what they want, which language, do they
| want a person — is a decision, not writing, so it goes to a decision
| model that answers fixed questions with probabilities instead of to a
| language model that writes text. One call asks every question about
| the same state at once. The pipeline never sees the vendor's API; a
| driver answers for one configured account.
|
*/

package contracts

import (
	"context"
	"time"
)

/*
Decider answers questions about one piece of state. It is safe for
concurrent use.
*/
type Decider interface {
	/*
		Decide asks every question of the request about its state and
		returns one answer per question name.
	*/
	Decide(ctx context.Context, req DecisionRequest) (*DecisionResponse, error)
}

/*
DecisionType names what a question asks for.
*/
type DecisionType string

/*
The question types: one option out of a set, a level on an ordered
scale, or the probability that a statement is true.
*/
const (
	DecisionChoice DecisionType = "choice"
	DecisionScore  DecisionType = "score"
	DecisionNoul   DecisionType = "noul"
)

/*
DecisionRequest is the state every question refers to, and the
questions by name.
*/
type DecisionRequest struct {
	State     string
	Questions map[string]DecisionQuestion
}

/*
DecisionQuestion is one question. Choices maps each option name of a
choice question to its description; Levels lists a score question's
levels, lowest first; True and False describe what makes a noul
statement true or false.
*/
type DecisionQuestion struct {
	Type         DecisionType
	Instructions string
	Choices      map[string]string
	Levels       []string
	True         string
	False        string
}

/*
DecisionResponse is the model that answered, its answers by question
name, its usage and how long it took.
*/
type DecisionResponse struct {
	Model   string
	Answers map[string]DecisionAnswer
	Usage   DecisionUsage
	Latency time.Duration
}

/*
DecisionAnswer is one answer: the chosen option with its confidence
and the probability of every option, a score, or for a noul the
probability that the statement is true.
*/
type DecisionAnswer struct {
	Type          DecisionType
	Choice        string
	Score         float64
	Confidence    float64
	Probabilities map[string]float64
	Noul          float64
}

/*
DecisionUsage is the token accounting of one call.
*/
type DecisionUsage struct {
	InputTokens  int64
	OutputTokens int64
}
