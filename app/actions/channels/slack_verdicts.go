/*
|--------------------------------------------------------------------------
| Verdicts The Slack Channel Asks For
|--------------------------------------------------------------------------
|
| Two questions the channel cannot answer by rules alone go to a model
| behind these interfaces: whether a chat message asks for support, and
| whether a draft the agent wrote gives the same answer as the reply a
| teammate sent. Without one, the channel falls back to rules that need
| no model.
|
*/

package channels

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
RequestClassifier decides whether a top-level message in a room asks
for support. Rooms whose trigger is ai ask it; without one, they fall
back to grouping by author.
*/
type RequestClassifier interface {
	/*
		Classify returns the verdict on the message.
	*/
	Classify(ctx context.Context, in ClassifyInput) (RequestVerdict, error)
}

/*
ClassifyInput is a message to classify: where it was written and what it
says, as untrusted text, with how eagerly the room wants requests found.
*/
type ClassifyInput struct {
	WorkspaceID  ids.UUID
	ConnectionID ids.UUID
	RoomID       ids.UUID
	Text         string
	Sensitivity  string
}

/*
RequestVerdict is a classifier's verdict: request, chatter or followup,
and how sure it is, from 0 to 1.
*/
type RequestVerdict struct {
	Kind       string
	Confidence float64
}

/*
DraftJudge compares a draft the agent wrote with the reply the teammate
sent instead.
*/
type DraftJudge interface {
	/*
		Agree reports whether the draft gives the same answer as the
		reply.
	*/
	Agree(ctx context.Context, in DraftComparison) (bool, error)
}

/*
DraftComparison is a draft and the reply sent instead, in their
conversation.
*/
type DraftComparison struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	Draft          string
	Reply          string
}
