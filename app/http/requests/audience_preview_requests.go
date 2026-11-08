/*
|--------------------------------------------------------------------------
| Audience Preview Requests
|--------------------------------------------------------------------------
|
| The inputs of counting who a rule matches and of testing a rule on one
| person, with what that person's browser would report typed in.
|
*/

package requests

import "encoding/json"

/*
PreviewAudienceRequest is the input of POST /audience-previews: a rule
and how many matching people to list.
*/
type PreviewAudienceRequest struct {
	Body struct {
		Rules  json.RawMessage `json:"rules"`
		Sample int             `json:"sample,omitempty" maximum:"25" minimum:"0"`
	}
}

/*
EvaluateAudienceRequest is the input of POST /audience-evaluations: a
rule, the person to test it on, and what their browser would report.
*/
type EvaluateAudienceRequest struct {
	Body struct {
		Rules     json.RawMessage `json:"rules"`
		ContactID string          `json:"contact_id"`
		Context   struct {
			PageURL string `json:"page_url,omitempty" maxLength:"2048"`
			Locale  string `json:"locale,omitempty"   maxLength:"35"`
			Device  string `json:"device,omitempty"   enum:"desktop,mobile,tablet"`
			Country string `json:"country,omitempty"  maxLength:"2"`
		} `json:"context,omitzero"`
	}
}
