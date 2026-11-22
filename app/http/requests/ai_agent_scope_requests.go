/*
|--------------------------------------------------------------------------
| AI Agent Scope Requests
|--------------------------------------------------------------------------
|
| What an agent may use and where it answers, as a save sends them,
| and the note a publish keeps.
|
*/

package requests

/*
AIScopeBody is what an agent may use of one kind: "all", or "selected"
with the public ids it names.
*/
type AIScopeBody struct {
	Mode string   `json:"mode"          validate:"required,oneof=all selected"`
	IDs  []string `json:"ids,omitempty" validate:"omitempty,max=200,dive,max=64"`
}

/*
AIKnowledgeScopeBody is what an agent may search: "all", or "selected"
with the help centers and knowledge sources it names.
*/
type AIKnowledgeScopeBody struct {
	Mode          string   `json:"mode"                      validate:"required,oneof=all selected"`
	HelpCenterIDs []string `json:"help_center_ids,omitempty" validate:"omitempty,max=50,dive,max=64"`
	SourceIDs     []string `json:"source_ids,omitempty"      validate:"omitempty,max=200,dive,max=64"`
}

/*
AIPlacementBody is where an agent takes new conversations: channel
connections (a widget, an address, a number), Slack channels and
inboxes, by public id.
*/
type AIPlacementBody struct {
	ChannelConnectionIDs []string `json:"channel_connection_ids,omitempty" validate:"omitempty,max=100,dive,max=64"`
	ChannelRoomIDs       []string `json:"channel_room_ids,omitempty"       validate:"omitempty,max=200,dive,max=64"`
	InboxIDs             []string `json:"inbox_ids,omitempty"              validate:"omitempty,max=100,dive,max=64"`
}

/*
AIPublishAgentBody is an optional note on what changed, kept on the
version.
*/
type AIPublishAgentBody struct {
	ChangeNote string `json:"change_note,omitempty" validate:"omitempty,max=500"`
}

/*
AIPublishAgentRequest is the input of POST
/ai/agents/{agent}/publications.
*/
type AIPublishAgentRequest struct {
	Agent string `path:"agent"`
	Body  *AIPublishAgentBody
}
