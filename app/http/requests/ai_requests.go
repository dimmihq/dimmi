/*
|--------------------------------------------------------------------------
| AI Requests
|--------------------------------------------------------------------------
|
| The agent's request shapes: its settings save whole, connections are
| created once and changed field by field, and the subscription
| connect carries only what a callback returns.
|
*/

package requests

/*
AIAgentRequest names the agent implicitly: the workspace has one.
*/
type AIAgentRequest struct{}

/*
AIUpdateAgentRequest is the input of PATCH /ai/agent.
*/
type AIUpdateAgentRequest struct {
	Body struct {
		Name             *string   `json:"name,omitempty"               validate:"omitempty,max=60"`
		Enabled          *bool     `json:"enabled,omitempty"`
		Tone             *string   `json:"tone,omitempty"               validate:"omitempty,oneof=friendly professional concise"`
		Guidance         *string   `json:"guidance,omitempty"           validate:"omitempty,max=2000"`
		ProductName      *string   `json:"product_name,omitempty"       validate:"omitempty,max=80"`
		EnabledChannels  *[]string `json:"enabled_channels,omitempty"`
		Schedule         *string   `json:"schedule,omitempty"           validate:"omitempty,oneof=always outside_office_hours inside_office_hours"`
		AnswerKeyID      *string   `json:"answer_key_id,omitempty"`
		AnswerModel      *string   `json:"answer_model,omitempty"       validate:"omitempty,max=120"`
		ClassifierKeyID  *string   `json:"classifier_key_id,omitempty"`
		ClassifierModel  *string   `json:"classifier_model,omitempty"   validate:"omitempty,max=120"`
		FallbackKeyID    *string   `json:"fallback_key_id,omitempty"`
		FallbackModel    *string   `json:"fallback_model,omitempty"     validate:"omitempty,max=120"`
		MonthlyCapUSD    *float64  `json:"monthly_cap_usd,omitempty"    validate:"omitempty,gte=0"`
		MaxAiReplies     *int      `json:"max_ai_replies,omitempty"     validate:"omitempty,gte=1,lte=50"`
		RequireCitations *bool     `json:"require_citations,omitempty"`
		PiiRedaction     *bool     `json:"pii_redaction,omitempty"`
		Languages        *[]string `json:"languages,omitempty"`
		DefaultLanguage  *string   `json:"default_language,omitempty"   validate:"omitempty,max=10"`
		UnknownBehavior  *string   `json:"unknown_behavior,omitempty"   validate:"omitempty,oneof=offer_handoff handoff"`
		AssignedBehavior *string   `json:"assigned_behavior,omitempty"  validate:"omitempty,oneof=skip_when_human_active never_when_assigned always"`
		Streaming        *bool     `json:"streaming,omitempty"`
		IncludeContact   *string   `json:"include_contact_context,omitempty" validate:"omitempty,oneof=basic none full"`
		HandoffTeamID    *string   `json:"handoff_team_id,omitempty"`
		RaisePriority    *bool     `json:"raise_priority_on_handoff,omitempty"`
	}
}

/*
AIKeyRequest names a connection in the path.
*/
type AIKeyRequest struct {
	Key string `path:"key"`
}

/*
AIStoreKeyRequest is the input of POST /ai/keys.
*/
type AIStoreKeyRequest struct {
	Body struct {
		Name     string         `json:"name"      validate:"required,max=60"`
		Provider string         `json:"provider"  validate:"required,max=40"`
		BaseURL  string         `json:"base_url"  validate:"omitempty,url,max=255"`
		APIKey   string         `json:"api_key"   validate:"omitempty,max=255"`
		Config   map[string]any `json:"config,omitempty"`
	}
}

/*
AIUpdateKeyRequest is the input of PATCH /ai/keys/{key}.
*/
type AIUpdateKeyRequest struct {
	Key  string `path:"key"`
	Body struct {
		Name    *string        `json:"name,omitempty"     validate:"omitempty,max=60"`
		BaseURL *string        `json:"base_url,omitempty" validate:"omitempty,url,max=255"`
		APIKey  *string        `json:"api_key,omitempty"  validate:"omitempty,max=255"`
		Config  map[string]any `json:"config,omitempty"`
	}
}

/*
AIChatGPTStartRequest is the input of POST /ai/chatgpt/connect.
*/
type AIChatGPTStartRequest struct{}

/*
AIChatGPTCompleteRequest is the input of POST /ai/chatgpt/callback.
*/
type AIChatGPTCompleteRequest struct {
	Body struct {
		State string `json:"state" validate:"required"`
		Code  string `json:"code"  validate:"required"`
	}
}

/*
AIRunsRequest is the input of GET .../conversations/:id/ai/runs.
*/
type AIRunsRequest struct {
	Conversation string `path:"conversation"`
}

/*
AIRunRequest is the input of GET .../conversations/:id/ai/runs/:run.
*/
type AIRunRequest struct {
	Conversation string `path:"conversation"`
	Run          string `path:"run"`
}
