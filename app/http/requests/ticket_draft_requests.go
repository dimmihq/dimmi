/*
|--------------------------------------------------------------------------
| Ticket Draft Requests
|--------------------------------------------------------------------------
|
| The inputs of the drafts closing the loop and of the workspace's
| ticket switches.
|
*/

package requests

/*
UpdateTicketUpdateDraftRequest sends or discards a drafted message.
*/
type UpdateTicketUpdateDraftRequest struct {
	Draft string `path:"draft"`
	Body  struct {
		Status  string  `json:"status"            enum:"sent,discarded" validate:"required"`
		Body    *string `json:"body,omitempty"    maxLength:"20000"`
		Resolve *bool   `json:"resolve,omitempty"`
	}
}

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
		IncidentMirror         *bool   `json:"incident_mirror,omitempty"`
		IncidentMirrorSeverity *string `json:"incident_mirror_severity,omitempty" enum:"all,minor,major,critical"`
		AutoLinkDuplicates     *bool   `json:"auto_link_duplicates,omitempty"`
		SpikeThreshold         *int32  `json:"spike_threshold,omitempty"          minimum:"2" maximum:"1000"`
	}
}
