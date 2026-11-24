/*
|--------------------------------------------------------------------------
| AI Settings Requests
|--------------------------------------------------------------------------
|
| The workspace's shared AI settings: the default models and the
| monthly spend cap every agent and Operator share.
|
*/

package requests

/*
AISettingsRequest is the input of GET /ai/settings.
*/
type AISettingsRequest struct{}

/*
AISettingsBody is the body of PATCH /ai/settings. Key ids are public
ids; a field sent as null is cleared; monthly_cap_usd of 0 or null
means no cap.
*/
type AISettingsBody struct {
	AnswerKeyID     *string  `json:"answer_key_id,omitempty"     validate:"omitempty,max=64"`
	AnswerModel     *string  `json:"answer_model,omitempty"      validate:"omitempty,max=120"`
	ClassifierKeyID *string  `json:"classifier_key_id,omitempty" validate:"omitempty,max=64"`
	ClassifierModel *string  `json:"classifier_model,omitempty"  validate:"omitempty,max=120"`
	FallbackKeyID   *string  `json:"fallback_key_id,omitempty"   validate:"omitempty,max=64"`
	FallbackModel   *string  `json:"fallback_model,omitempty"    validate:"omitempty,max=120"`
	MonthlyCapUSD   *float64 `json:"monthly_cap_usd,omitempty"   validate:"omitempty,gte=0,lte=100000"`
}

/*
AIUpdateSettingsRequest is the input of PATCH /ai/settings. RawBody
tells a field sent as null from one left out.
*/
type AIUpdateSettingsRequest struct {
	Body    AISettingsBody
	RawBody []byte
}
