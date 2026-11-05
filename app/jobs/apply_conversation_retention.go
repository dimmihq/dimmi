/*
|--------------------------------------------------------------------------
| Apply Conversation Retention
|--------------------------------------------------------------------------
|
| The daily round that moves closed conversations past their workspace's
| period to the trash.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/retention"
)

/*
ApplyConversationRetentionArgs names the job; it takes no arguments.
*/
type ApplyConversationRetentionArgs struct{}

/*
Kind names the job.
*/
func (ApplyConversationRetentionArgs) Kind() string { return "apply_conversation_retention" }

/*
Options run one round a day, tried three times.
*/
func (ApplyConversationRetentionArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: 24 * time.Hour}}
}

/*
ApplyConversationRetention runs the round.
*/
type ApplyConversationRetention struct {
	Apply *retention.ApplyConversationRetention
}

/*
Handle runs the round.
*/
func (w *ApplyConversationRetention) Handle(ctx context.Context, _ *queue.Job[ApplyConversationRetentionArgs]) error {
	_, err := w.Apply.Handle(ctx)

	return err
}
