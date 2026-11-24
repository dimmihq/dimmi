/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Jobs
|--------------------------------------------------------------------------
|
| A customer's message asks for the conversation's AI attributes to be
| checked again, a little later, so a burst of messages is read once.
| One check waits per conversation at a time.
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
AttributeDetector checks a conversation's AI attributes.
*/
type AttributeDetector interface {
	/*
		Detect reads the conversation and keeps the values it finds.
	*/
	Detect(ctx context.Context, args DetectConversationAttributesArgs) error
}

/*
DetectConversationAttributesArgs names the conversation to check, not
before NotBefore, which is left out of what makes the job unique.
*/
type DetectConversationAttributesArgs struct {
	WorkspaceID    ids.UUID  `json:"workspace_id"`
	ConversationID ids.UUID  `json:"conversation_id"`
	NotBefore      time.Time `json:"-"`
}

/*
Kind names the job.
*/
func (DetectConversationAttributesArgs) Kind() string { return "detect_conversation_attributes" }

/*
Options keep one waiting check per conversation, run no sooner than
asked.
*/
func (a DetectConversationAttributesArgs) Options() queue.Options {
	return queue.Options{
		ScheduledAt: a.NotBefore, MaxAttempts: 3,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()},
	}
}

/*
DetectConversationAttributes runs one check.
*/
type DetectConversationAttributes struct {
	Detector AttributeDetector
}

/*
Handle runs the check.
*/
func (w *DetectConversationAttributes) Handle(ctx context.Context, job *queue.Job[DetectConversationAttributesArgs],
) error {
	return w.Detector.Detect(ctx, job.Args)
}
