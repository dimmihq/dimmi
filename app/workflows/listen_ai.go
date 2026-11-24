/*
|--------------------------------------------------------------------------
| AI Attribute Triggers
|--------------------------------------------------------------------------
|
| The decision model setting or changing an AI attribute of a
| conversation starts the workflows of that trigger, which read which
| attribute changed and what it became.
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
listenAI matches AI attribute changes.
*/
func (e *Engine) listenAI(d *events.Dispatcher) {
	events.ListenQueued(d, func(ev appevents.ConversationAIAttributeChanged) queue.Args {
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerAIAttributeChanged, map[string]any{
			"ai_attribute.key": ev.Key, "ai_attribute.value": ev.To,
		})
	})
}
