/*
|--------------------------------------------------------------------------
| Knowledge Source Events
|--------------------------------------------------------------------------
|
| A source's sync ends one of two ways: it completed, fully or with some
| documents failing, or it failed as a whole. Listeners and webhooks
| learn which, with the run's counters or the error's code; neither
| ever carries the source's credentials.
|
*/

package events

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
KnowledgeSourceSyncCompleted is dispatched in the transaction that ended
a sync that read its source. Status is succeeded, partial or canceled,
and Stats holds the run's counters.
*/
type KnowledgeSourceSyncCompleted struct {
	Base
	SourceID ids.UUID
	RunID    ids.UUID
	Status   string
	Stats    json.RawMessage
}

/*
EventName names the event.
*/
func (KnowledgeSourceSyncCompleted) EventName() string {
	return "knowledge_source.sync_completed"
}

/*
KnowledgeSourceSyncFailed is dispatched in the transaction that ended a
sync that could not read its source. Code says why, such as auth_failed.
*/
type KnowledgeSourceSyncFailed struct {
	Base
	SourceID ids.UUID
	RunID    ids.UUID
	Code     string
}

/*
EventName names the event.
*/
func (KnowledgeSourceSyncFailed) EventName() string {
	return "knowledge_source.sync_failed"
}
