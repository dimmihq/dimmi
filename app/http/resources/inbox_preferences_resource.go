/*
|--------------------------------------------------------------------------
| Inbox Preferences Resource
|--------------------------------------------------------------------------
|
| A member's inbox preferences as the API shows them: the sidebar, each
| view's sort, status, layout and columns, the pinned attributes and when
| they were last saved.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
InboxPreferences is a member's inbox preferences.
*/
type InboxPreferences struct {
	Object           string                                  `json:"object"`
	Sidebar          json.RawMessage                         `json:"sidebar"`
	Views            map[string]conversations.ViewPreference `json:"views"`
	PinnedAttributes []string                                `json:"pinned_attributes"`
	UpdatedAt        *time.Time                              `json:"updated_at"`
}

/*
NewInboxPreferences shapes the preferences.
*/
func NewInboxPreferences(p conversations.Preferences) InboxPreferences {
	return InboxPreferences{
		Object: "inbox_preferences", Sidebar: p.Sidebar, Views: p.Views, PinnedAttributes: p.PinnedAttributes,
		UpdatedAt: p.UpdatedAt,
	}
}
