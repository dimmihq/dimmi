/*
|--------------------------------------------------------------------------
| Ticket Settings Requests
|--------------------------------------------------------------------------
|
| The inputs of the workspace's ticket switches.
|
*/

package requests

/*
TicketSettingsRequest reads the workspace's ticket switches.
*/
type TicketSettingsRequest struct{}

/*
UpdateTicketSettingsRequest changes the workspace's ticket switches;
an absent one stays.
*/
type UpdateTicketSettingsRequest struct {
	Body struct {
		AutoLinkDuplicates *bool `json:"auto_link_duplicates,omitempty"`
	}
}
