/*
|--------------------------------------------------------------------------
| Event Settings Requests
|--------------------------------------------------------------------------
|
| The input of setting whether the messenger may send any event name.
|
*/

package requests

/*
UpdateEventSettingsRequest is the input of PATCH /event-settings.
*/
type UpdateEventSettingsRequest struct {
	Body struct {
		// MessengerNewEvents lets the messenger send any event name,
		// defining the ones the workspace has not seen.
		MessengerNewEvents bool `json:"messenger_new_events"`
	}
}
