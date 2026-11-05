/*
|--------------------------------------------------------------------------
| Redaction Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on redaction rules. A pattern is checked again
| by compiling it; the length limits here keep a request small. A text to
| try is bounded like a message body.
|
*/

package requests

/*
RedactionRuleRequest is the input of the endpoints that name only a
rule.
*/
type RedactionRuleRequest struct {
	Rule string `path:"redaction_rule"`
}

/*
StoreRedactionRuleRequest is the input of POST /redaction-rules.
*/
type StoreRedactionRuleRequest struct {
	Body struct {
		Name        string  `json:"name"                  validate:"required,min=1,max=80"`
		Pattern     string  `json:"pattern"               validate:"required,min=1,max=256"`
		Replacement *string `json:"replacement,omitempty" validate:"omitempty,max=80"`
		Enabled     *bool   `json:"enabled,omitempty"`
	}
}

/*
UpdateRedactionRuleRequest is the input of PATCH
/redaction-rules/{redaction_rule}.
*/
type UpdateRedactionRuleRequest struct {
	Rule string `path:"redaction_rule"`
	Body struct {
		Name        *string `json:"name,omitempty"        validate:"omitempty,min=1,max=80"`
		Pattern     *string `json:"pattern,omitempty"     validate:"omitempty,min=1,max=256"`
		Replacement *string `json:"replacement,omitempty" validate:"omitempty,max=80"`
		Enabled     *bool   `json:"enabled,omitempty"`
	}
}

/*
UpdateRedactionSettingsRequest is the input of PATCH /redaction-rules:
the built-in switches and the order of the workspace's rules by id.
*/
type UpdateRedactionSettingsRequest struct {
	Body struct {
		CardNumbers *bool    `json:"card_numbers,omitempty"`
		USSSN       *bool    `json:"us_ssn,omitempty"`
		Order       []string `json:"order,omitempty"        validate:"omitempty,max=10"`
	}
}

/*
TryRedactionRequest is the input of POST /redaction-tests.
*/
type TryRedactionRequest struct {
	Body struct {
		Text string `json:"text" validate:"required,max=65536"`
	}
}
