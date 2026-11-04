/*
|--------------------------------------------------------------------------
| Inbox Preference Events
|--------------------------------------------------------------------------
|
| A member's inbox preferences are theirs alone: saving them tells only
| that member, so their other tabs follow the same sort and sidebar.
|
*/

package events

/*
InboxPreferencesUpdated is dispatched when a member saves their inbox
preferences.
*/
type InboxPreferencesUpdated struct {
	Base
	MemberRef
}

/*
EventName names the event.
*/
func (InboxPreferencesUpdated) EventName() string { return "inbox_preferences.updated" }
