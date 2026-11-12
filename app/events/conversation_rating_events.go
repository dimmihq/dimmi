/*
|--------------------------------------------------------------------------
| Conversation Rating Events
|--------------------------------------------------------------------------
|
| A closed conversation asks its customer how it went, and the customer
| answers with a score from one to five and maybe a comment. The ask
| going out tells the visitor's messenger to show the rating card; the
| answer reaches the timeline, webhooks and workflows.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationRatingRequested is dispatched when the ask for a rating
goes out to the customer. Channel is widget or email.
*/
type ConversationRatingRequested struct {
	Base
	ConversationRef

	RatingID ids.UUID
	Channel  string
}

/*
EventName names the event.
*/
func (ConversationRatingRequested) EventName() string { return "conversation.rating_requested" }

/*
ConversationRated is dispatched when a customer rates a conversation or
changes their rating. Score is from 1 to 5; Comment is what they wrote,
empty for none. First is true the first time this ask is answered.
MemberID is the teammate the conversation was assigned to at the
close, nil for nobody.
*/
type ConversationRated struct {
	Base
	ConversationRef

	RatingID ids.UUID
	Score    int
	Comment  string
	Channel  string
	MemberID *ids.UUID
	First    bool
}

/*
EventName names the event.
*/
func (ConversationRated) EventName() string { return "conversation.rated" }
