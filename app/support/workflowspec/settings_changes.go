/*
|--------------------------------------------------------------------------
| State And Outside Step Settings
|--------------------------------------------------------------------------
|
| The typed settings of the steps that change a contact, snooze, close or
| time a conversation, or call another service; see the step settings
| file for how ids and optional switches are written.
|
*/

package workflowspec

/*
UpdateContactSettings sets contact fields.
*/
type UpdateContactSettings struct {
	Set []ContactChange `json:"set"`
}

/*
SnoozeSettings snoozes the conversation For a duration or Until the
schedule's next opening.
*/
type SnoozeSettings struct {
	For   string     `json:"for,omitempty"`
	Until *WaitUntil `json:"until,omitempty"`
}

/*
CloseSettings closes the conversation. OnlyIfNoTeammateReply skips
closing conversations a teammate has answered.
*/
type CloseSettings struct {
	AskRating             bool `json:"ask_rating,omitempty"`
	OnlyIfNoTeammateReply bool `json:"only_if_no_teammate_reply,omitempty"`
}

/*
ApplySLASettings applies an SLA policy.
*/
type ApplySLASettings struct {
	SLAPolicyID string `json:"sla_policy_id"`
}

/*
HTTPRequestSettings calls the customer's own system.
*/
type HTTPRequestSettings struct {
	URL    string `json:"url,omitempty"`
	Method string `json:"method,omitempty"`
}
