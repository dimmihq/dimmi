/*
|--------------------------------------------------------------------------
| Conversation Rating Mail Job
|--------------------------------------------------------------------------
|
| An email ask goes out from a job of its own, outside any transaction,
| the way replies do. It is unique while pending, so a repeated event
| never mails twice.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
MailConversationRatingArgs names an ask whose email goes out, from the
address the conversation's replies use.
*/
type MailConversationRatingArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RatingID    ids.UUID `json:"rating_id"`
}

/*
Kind names the job.
*/
func (MailConversationRatingArgs) Kind() string { return "mail_conversation_rating" }

/*
Options keep one pending job per ask.
*/
func (MailConversationRatingArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
MailConversationRating emails a rating ask.
*/
type MailConversationRating struct {
	Asker RatingAsker
}

/*
Handle emails the ask.
*/
func (w *MailConversationRating) Handle(ctx context.Context, job *queue.Job[MailConversationRatingArgs]) error {
	return w.Asker.Mail(ctx, job.Args)
}
