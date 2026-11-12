/*
|--------------------------------------------------------------------------
| AI Questions
|--------------------------------------------------------------------------
|
| What the model reads to match a conversation to the trackers and
| incidents it may report, and to group a burst of conversations by
| what they are about.
|
*/

package tickets

import "github.com/dimmihq/dimmi/framework/ids"

/*
MatchCandidate is an open tracker or incident a report may belong to.
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

/*
GroupItem is one conversation to group.
*/
type GroupItem struct {
	ID   ids.UUID
	Text string
}

/*
GroupQuestion is the conversations of a window.
*/
type GroupQuestion struct {
	WorkspaceID ids.UUID
	Items       []GroupItem
}

/*
Group is a set of conversations about one thing: a short title, the
affected area when one is clear, and a stable key for the topic.
*/
type Group struct {
	Key   string
	Title string
	Area  string
	IDs   []ids.UUID
}
