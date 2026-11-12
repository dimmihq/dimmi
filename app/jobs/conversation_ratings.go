/*
|--------------------------------------------------------------------------
| Conversation Rating Jobs
|--------------------------------------------------------------------------
|
| A close becomes a rating ask from a job, so the close itself never
| waits on the settings or the customer's address. The ask goes out
| from a second job when it comes due, straight away or after the delay
| the workspace chose; by then the conversation may have opened again,
| and then nothing is sent. Each is unique while pending, so a repeated
| event never asks twice.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
AskConversationRatingArgs names a close that may ask for a rating.
Actor is who closed it, and Forced is true when a workflow's close step
asked, whatever the settings say.
*/
type AskConversationRatingArgs struct {
	WorkspaceID    ids.UUID  `json:"workspace_id"`
	ConversationID ids.UUID  `json:"conversation_id"`
	ClosedAt       time.Time `json:"closed_at"`
	Actor          string    `json:"actor"`
	Forced         bool      `json:"forced,omitempty"`
}

/*
Kind names the job.
*/
func (AskConversationRatingArgs) Kind() string { return "ask_conversation_rating" }

/*
Options keep one pending job per close.
*/
func (AskConversationRatingArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
SendConversationRatingArgs names an ask that is due.
*/
type SendConversationRatingArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RatingID    ids.UUID `json:"rating_id"`
}

/*
Kind names the job.
*/
func (SendConversationRatingArgs) Kind() string { return "send_conversation_rating" }

/*
Options keep one pending job per ask.
*/
func (SendConversationRatingArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
RatingAsker opens and sends rating asks.
*/
type RatingAsker interface {
	/*
		Ask opens the ask of a close, when the close asks for one.
	*/
	Ask(ctx context.Context, args AskConversationRatingArgs) error

	/*
		Send sends a due ask to the customer.
	*/
	Send(ctx context.Context, args SendConversationRatingArgs) error

	/*
		Mail emails an ask to an email conversation's customer.
	*/
	Mail(ctx context.Context, args MailConversationRatingArgs) error
}

/*
AskConversationRating opens a close's rating ask.
*/
type AskConversationRating struct {
	Asker RatingAsker
}

/*
Handle opens the ask.
*/
func (w *AskConversationRating) Handle(ctx context.Context, job *queue.Job[AskConversationRatingArgs]) error {
	return w.Asker.Ask(ctx, job.Args)
}

/*
SendConversationRating sends a due rating ask.
*/
type SendConversationRating struct {
	Asker RatingAsker
}

/*
Handle sends the ask.
*/
func (w *SendConversationRating) Handle(ctx context.Context, job *queue.Job[SendConversationRatingArgs]) error {
	return w.Asker.Send(ctx, job.Args)
}
