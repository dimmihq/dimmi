/*
|--------------------------------------------------------------------------
| Tag Events
|--------------------------------------------------------------------------
|
| The workspace's vocabulary changed: a tag was added, renamed or
| recolored, or removed. Open screens hear about it on the workspace's
| channel so pickers and chips stay current without a reload. Deleting a
| tag is one event, however many records carried it, and says how many.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WorkspaceEvent is an event every member viewing the workspace hears
about, carrying the data it names.
*/
type WorkspaceEvent interface {
	Named

	/*
		BroadcastData returns what the event carries to open screens.
	*/
	BroadcastData() any
}

/*
TagCreated is dispatched when a tag is added to the vocabulary.
*/
type TagCreated struct {
	Base

	TagID ids.UUID
	Data  any
}

/*
EventName names the event.
*/
func (TagCreated) EventName() string { return "tag.created" }

/*
BroadcastData returns the tag as open screens read it.
*/
func (e TagCreated) BroadcastData() any { return e.Data }

/*
TagUpdated is dispatched when a tag is renamed or recolored. Changes
lists the fields that changed.
*/
type TagUpdated struct {
	Base

	TagID   ids.UUID
	Changes []string
	Data    any
}

/*
EventName names the event.
*/
func (TagUpdated) EventName() string { return "tag.updated" }

/*
BroadcastData returns the tag as open screens read it.
*/
func (e TagUpdated) BroadcastData() any { return e.Data }

/*
TagDeleted is dispatched when a tag is removed, once, with how many
conversations, contacts and companies carried it.
*/
type TagDeleted struct {
	Base

	TagID         ids.UUID
	Conversations int64
	Contacts      int64
	Companies     int64
	Data          any
}

/*
EventName names the event.
*/
func (TagDeleted) EventName() string { return "tag.deleted" }

/*
BroadcastData returns what open screens need to drop the tag.
*/
func (e TagDeleted) BroadcastData() any { return e.Data }
