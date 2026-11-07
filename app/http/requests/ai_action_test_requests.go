/*
|--------------------------------------------------------------------------
| Agent Action Test Requests
|--------------------------------------------------------------------------
|
| Publishing an action, and trying an action or a connection before the
| agent relies on it.
|
*/

package requests

import (
	"encoding/json"
)

/*
AIPublishActionRequest is the input of POST
/ai/actions/{action}/publications.
*/
type AIPublishActionRequest struct {
	Action string `path:"action"`
	Body   struct {
		Note                string   `json:"note,omitempty"                 validate:"omitempty,max=500"`
		AcknowledgeWarnings []string `json:"acknowledge_warnings,omitempty"`
	}
}

/*
AITestActionRequest is the input of POST /ai/actions/{action}/tests.
*/
type AITestActionRequest struct {
	Action string `path:"action"`
	Body   struct {
		Input              json.RawMessage `json:"input,omitempty"`
		Contact            *AITestContact  `json:"contact,omitempty"`
		Mode               string          `json:"mode,omitempty"                 validate:"omitempty,max=16"`
		Version            string          `json:"version,omitempty"              validate:"omitempty,max=16"`
		ConfirmSideEffects bool            `json:"confirm_side_effects,omitempty"`
	}
}

/*
AITestContact is the stand-in customer of a test, treated as verified;
custom_attributes and attributes are the same values.
*/
type AITestContact struct {
	ExternalID       string         `json:"external_id,omitempty"       validate:"omitempty,max=256"`
	Email            string         `json:"email,omitempty"             validate:"omitempty,max=320"`
	Attributes       map[string]any `json:"attributes,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

/*
AITestActionConnectionRequest is the input of POST
/ai/action-connections/{connection}/tests: an optional path to probe
under the base URL.
*/
type AITestActionConnectionRequest struct {
	Connection string `path:"connection"`
	Body       struct {
		Path string `json:"path,omitempty" validate:"omitempty,max=512"`
	}
}
