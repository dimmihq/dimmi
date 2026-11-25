/*
|--------------------------------------------------------------------------
| Procedure Simulator Requests
|--------------------------------------------------------------------------
|
| The made-up customer a simulated turn or a saved test talks as, the
| earlier messages of the conversation, and what a mocked action
| answers.
|
*/

package requests

import "encoding/json"

/*
SimMessageInput is one earlier message of a simulated conversation.
*/
type SimMessageInput struct {
	Role string `json:"role" enum:"customer,agent"`
	Text string `json:"text" maxLength:"4000"`
}

/*
SimPersonaInput is the made-up customer of a simulation or a test.
*/
type SimPersonaInput struct {
	Name       string         `json:"name,omitempty"        maxLength:"200"`
	Email      string         `json:"email,omitempty"       maxLength:"320"`
	ExternalID string         `json:"external_id,omitempty" maxLength:"200"`
	Verified   bool           `json:"verified,omitempty"`
	Channel    string         `json:"channel,omitempty"     enum:"widget,email,slack,whatsapp,instagram,facebook,api,"`
	InboxID    string         `json:"inbox_id,omitempty"    maxLength:"64"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Company    map[string]any `json:"company,omitempty"`
}

/*
ActionMockInput is what a mocked action answers: an HTTP status and a
body.
*/
type ActionMockInput struct {
	Status int             `json:"status,omitempty" minimum:"100" maximum:"599"`
	Body   json.RawMessage `json:"body,omitempty"`
}
