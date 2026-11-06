/*
|--------------------------------------------------------------------------
| AI Playground Contract
|--------------------------------------------------------------------------
|
| A teammate asks the agent a question the way a customer would and
| sees everything it did: how it read the question, what it found,
| what it would answer and what that cost. Nothing reaches a customer:
| the playground never writes a message, never changes a conversation
| and never counts toward billing, though it records its run and its
| spend counts toward the workspace's cap. The pipeline implements it.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
AIPlayground answers a test question with the agent's full pipeline,
writing nothing a customer sees.
*/
type AIPlayground interface {
	/*
		Ask runs the question through classification, retrieval and
		generation in the workspace, reading the conversation's history
		when one is named, and returns what the agent would do.
	*/
	Ask(ctx context.Context, q PlaygroundQuestion) (*PlaygroundAnswer, error)
}

/*
PlaygroundQuestion is one test question: the workspace and member
asking, the text, and optionally a conversation whose history the agent
reads as context.
*/
type PlaygroundQuestion struct {
	WorkspaceID    ids.UUID
	MemberID       ids.UUID
	Message        string
	ConversationID *ids.UUID
}

/*
PlaygroundAnswer is what the agent would do. RunID names the recorded
playground run when one was kept; Outcome is how the run ended
(answered, unknown, handoff); Answer is the reply it would send, empty
when it would send none. The token counts and CostMicros, in
micro-dollars, are what the run consumed.
*/
type PlaygroundAnswer struct {
	RunID             *ids.UUID
	Outcome           string
	Classification    PlaygroundClassification
	Retrieved         []PlaygroundSource
	Answer            string
	Citations         []PlaygroundCitation
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	CostMicros        int64
	FirstTokenMs      int
	LatencyMs         int
}

/*
PlaygroundClassification is how the agent read the question: its
intent, its language and the standalone query it searched with.
*/
type PlaygroundClassification struct {
	Intent   string
	Language string
	Query    string
}

/*
PlaygroundSource is one retrieved passage, numbered as the answer
cites it, with whether the answer used it.
*/
type PlaygroundSource struct {
	N          int
	Title      string
	URL        string
	Similarity float64
	Used       bool
	Text       string
}

/*
PlaygroundCitation is one source the answer cites.
*/
type PlaygroundCitation struct {
	N     int
	Title string
	URL   string
}
