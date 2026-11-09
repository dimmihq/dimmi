/*
|--------------------------------------------------------------------------
| Streaming To The Teammate
|--------------------------------------------------------------------------
|
| A run reaches only the teammate who asked, on their own channel: the
| text as the model writes it and each step as it finishes, neither kept
| for replay, then the stored answer, which is. A broadcast that fails
| changes nothing: the request answers with the stored message anyway.
|
*/

package operator

import (
	"context"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The events a run sends its teammate.
*/
const (
	EventDelta    = "operator.delta"
	EventStep     = "operator.step"
	EventAnswered = "operator.answered"
)

/*
publishDelta sends a piece of the answer's text.
*/
func (e *Engine) publishDelta(ctx context.Context, s *session, text string) {
	if text == "" {
		return
	}
	e.publish(ctx, s, fbroadcasting.Message{
		Event: EventDelta, Ephemeral: true,
		Data: map[string]string{"thread_id": threadID(s), "text": text},
	})
}

/*
publishStep sends a finished step.
*/
func (e *Engine) publishStep(ctx context.Context, s *session, st Step) {
	e.publish(ctx, s, fbroadcasting.Message{
		Event: EventStep, Ephemeral: true, Data: map[string]any{"thread_id": threadID(s), "step": st},
	})
}

/*
publishAnswered tells the teammate's other screens the thread has a new
answer to fetch.
*/
func (e *Engine) publishAnswered(ctx context.Context, s *session, msg models.OperatorMessage) {
	e.publish(ctx, s, fbroadcasting.Message{
		ID: ids.Encode("evt", ids.New()), Event: EventAnswered,
		Data: map[string]string{
			"thread_id": threadID(s), "message_id": models.PublicID(models.PrefixOperatorMessage, msg.ID),
		},
	})
}

/*
publish sends one broadcast on the asking teammate's channel.
*/
func (e *Engine) publish(ctx context.Context, s *session, m fbroadcasting.Message) {
	if e.Publisher == nil || s.background {
		return
	}
	_ = e.Publisher.Publish(ctx, []string{appbroadcasting.MemberChannel(s.m.Member.ID)}, m)
}

/*
threadID is the session's thread as clients name it.
*/
func threadID(s *session) string {
	return models.PublicID(models.PrefixOperatorThread, s.thread.ID)
}
