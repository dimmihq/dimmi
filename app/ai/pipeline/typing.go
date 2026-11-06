/*
|--------------------------------------------------------------------------
| Typing Indicator
|--------------------------------------------------------------------------
|
| While the answer model writes, the customer's messenger shows the
| agent typing, through the same ephemeral typing events a teammate's
| typing travels on, so the customer waits for the answer instead of
| leaving. Only the widget shows typing; nothing is stored.
|
*/

package pipeline

import (
	"context"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
typingPayload is what the widget learns about who is typing.
*/
type typingPayload struct {
	ConversationID string  `json:"conversation_id"`
	Name           string  `json:"name"`
	AvatarURL      *string `json:"avatar_url"`
}

/*
typing announces the agent typing and returns the call that stops it.
*/
func (r *Runner) typing(ctx context.Context, e *env) func() {
	if r.Publisher == nil || e.ChannelType != "widget" {
		return func() {}
	}
	r.publishTyping(ctx, e, "typing.started")
	return func() { r.publishTyping(context.WithoutCancel(ctx), e, "typing.stopped") }
}

/*
publishTyping sends one typing event to the customer's widget; a
failed relay only costs the indicator.
*/
func (r *Runner) publishTyping(ctx context.Context, e *env, event string) {
	_ = r.Publisher.Publish(ctx, []string{appbroadcasting.WidgetContactChannel(e.Conversation.ContactID)},
		fbroadcasting.Message{
			ID: ids.Encode("evt", ids.New()), Event: event, Ephemeral: true,
			Data: typingPayload{
				ConversationID: models.PublicID(models.PrefixConversation, e.Conversation.ID),
				Name:           e.Agent.Name,
			},
		})
}
