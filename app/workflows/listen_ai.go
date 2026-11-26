/*
|--------------------------------------------------------------------------
| Conversation Attribute Triggers
|--------------------------------------------------------------------------
|
| A conversation attribute set, changed or cleared, by anyone, starts
| the workflows of that trigger, which read which attribute changed,
| what it became and who set it. The AI setting one also starts those
| of the AI's own trigger.
|
*/

package workflows

import (
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
listenAI matches conversation attribute changes: any change, and the
AI's own under the trigger that started on them before.
*/
func (e *Engine) listenAI(d *events.Dispatcher) {
	events.ListenQueued(d, func(ev appevents.ConversationAttributeChanged) queue.Args {
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerAttributeChanged, map[string]any{
			"attribute.key": ev.Key, "attribute.value": ev.To, "attribute.by": ev.By,
		})
	})

	events.ListenQueued(d, func(ev appevents.ConversationAIAttributeChanged) queue.Args {
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerAIAttributeChanged, map[string]any{
			"ai_attribute.key": ev.Key, "ai_attribute.value": ev.To,
		})
	})
}
