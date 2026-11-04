/*
|--------------------------------------------------------------------------
| Inbox Preference Requests
|--------------------------------------------------------------------------
|
| A member saves the parts of their inbox preferences they changed: the
| sidebar, the views or the pinned attributes. A part left out keeps what
| was saved; a part sent replaces the saved one whole.
|
*/

package requests

/*
ViewPreferenceBody is how a member likes one view.
*/
type ViewPreferenceBody struct {
	Sort    string   `json:"sort,omitempty"`
	Status  string   `json:"status,omitempty"`
	Layout  string   `json:"layout,omitempty"`
	Columns []string `json:"columns,omitempty" maxItems:"40"`
}

/*
UpdateInboxPreferencesRequest is the input of PATCH /inbox-preferences.
*/
type UpdateInboxPreferencesRequest struct {
	Body struct {
		Sidebar          map[string]any                `json:"sidebar,omitempty"`
		Views            map[string]ViewPreferenceBody `json:"views,omitempty"`
		PinnedAttributes []string                      `json:"pinned_attributes,omitempty" maxItems:"50"`
	}
}
