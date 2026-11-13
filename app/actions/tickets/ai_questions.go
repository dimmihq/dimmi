/*
|--------------------------------------------------------------------------
| AI Questions
|--------------------------------------------------------------------------
|
| What the model reads to match a conversation to the trackers it may
| report.
|
*/

package tickets

import "github.com/dimmihq/dimmi/framework/ids"

/*
MatchCandidate is an open tracker a report may belong to.
*/
type MatchCandidate struct {
	ID          ids.UUID
	Title       string
	Description string
}

/*
MatchQuestion is a conversation's words and the candidates.
*/
type MatchQuestion struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	Text           string
	Candidates     []MatchCandidate
}
