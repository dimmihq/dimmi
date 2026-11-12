/*
|--------------------------------------------------------------------------
| Changelog Events
|--------------------------------------------------------------------------
|
| A changelog entry going live, being taken back, or having its live copy
| replaced by a newer one. Each is dispatched in the transaction that
| changed what readers see, so webhooks hear it only when it happened.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ChangelogEntryPublished says an entry went live for the first time
since it was a draft, by a teammate or on its schedule.
*/
type ChangelogEntryPublished struct {
	Base
	EntryID ids.UUID
}

/*
EventName names the event.
*/
func (ChangelogEntryPublished) EventName() string { return "changelog_entry.published" }

/*
ChangelogEntryUnpublished says a live or scheduled entry went back to
a draft.
*/
type ChangelogEntryUnpublished struct {
	Base
	EntryID ids.UUID
}

/*
EventName names the event.
*/
func (ChangelogEntryUnpublished) EventName() string { return "changelog_entry.unpublished" }

/*
ChangelogEntryUpdated says an entry already live was published again,
so readers see its edits.
*/
type ChangelogEntryUpdated struct {
	Base
	EntryID ids.UUID
}

/*
EventName names the event.
*/
func (ChangelogEntryUpdated) EventName() string { return "changelog_entry.updated" }
