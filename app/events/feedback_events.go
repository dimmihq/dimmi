/*
|--------------------------------------------------------------------------
| Feedback Events
|--------------------------------------------------------------------------
|
| What happens to a feature request: it is posted, it moves to another
| status, a customer votes for it, a duplicate is merged into it, or a
| conversation is linked to it. Each is dispatched in the transaction
| that made the change.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
FeedbackPostCreated says a request was posted, by the team, from a
conversation or by a customer on the board.
*/
type FeedbackPostCreated struct {
	Base
	PostID ids.UUID
}

/*
EventName names the event.
*/
func (FeedbackPostCreated) EventName() string { return "feedback_post.created" }

/*
FeedbackPostStatusChanged says a request moved from one status key to
another.
*/
type FeedbackPostStatusChanged struct {
	Base
	PostID ids.UUID
	From   string
	To     string
}

/*
EventName names the event.
*/
func (FeedbackPostStatusChanged) EventName() string { return "feedback_post.status_changed" }

/*
FeedbackPostVoted says a customer's vote for a request was counted,
cast by them on the board or by a teammate for them. MemberID is the
teammate, nil when the customer voted.
*/
type FeedbackPostVoted struct {
	Base
	PostID    ids.UUID
	ContactID ids.UUID
	MemberID  *ids.UUID
	Priority  string
}

/*
EventName names the event.
*/
func (FeedbackPostVoted) EventName() string { return "feedback_post.voted" }

/*
FeedbackPostMerged says the source request, a duplicate, was folded
into the request kept.
*/
type FeedbackPostMerged struct {
	Base
	PostID   ids.UUID
	SourceID ids.UUID
}

/*
EventName names the event.
*/
func (FeedbackPostMerged) EventName() string { return "feedback_post.merged" }

/*
FeedbackPostLinked says a conversation was linked to a request for the
first time.
*/
type FeedbackPostLinked struct {
	Base
	PostID         ids.UUID
	ConversationID ids.UUID
}

/*
EventName names the event.
*/
func (FeedbackPostLinked) EventName() string { return "feedback_post.linked_conversation" }
