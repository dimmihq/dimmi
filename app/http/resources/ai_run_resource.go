/*
|--------------------------------------------------------------------------
| Agent Run Resource
|--------------------------------------------------------------------------
|
| One run and, when asked, its steps: the trace a teammate opens from
| a conversation's "why". Payloads are the redacted records the
| pipeline stored, trimmed to what the panel shows.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
AiRun is one run of the agent.
*/
type AiRun struct {
	ID              string            `json:"id"`
	ConversationID  *string           `json:"conversation_id"`
	Kind            string            `json:"kind"`
	Status          string            `json:"status"`
	Outcome         *string           `json:"outcome"`
	SkipReason      *string           `json:"skip_reason"`
	HandoffReason   *string           `json:"handoff_reason"`
	Language        *string           `json:"language"`
	AnswerMessageID *string           `json:"answer_message_id"`
	InputTokens     int64             `json:"input_tokens"`
	OutputTokens    int64             `json:"output_tokens"`
	CostMicros      int64             `json:"cost_micros"`
	LatencyMs       *int32            `json:"latency_ms"`
	FirstTokenMs    *int32            `json:"first_token_ms"`
	PromptVersion   string            `json:"prompt_version"`
	Flags           []string          `json:"flags"`
	Error           any               `json:"error"`
	Steps           []AiRunStep       `json:"steps,omitempty"`
	CreatedAt       any               `json:"created_at"`
}

/*
NewAiRun shapes a run for the API.
*/
func NewAiRun(r models.AiRun) AiRun {
	return AiRun{
		ID:              models.PublicID(models.PrefixAiRun, r.ID),
		ConversationID:  publicPtr(models.PrefixConversation, r.ConversationID),
		Kind:            r.Kind,
		Status:          r.Status,
		Outcome:         r.Outcome,
		SkipReason:      r.SkipReason,
		HandoffReason:   r.HandoffReason,
		Language:        r.Language,
		AnswerMessageID: publicPtr(models.PrefixMessage, r.AnswerMessageID),
		InputTokens:     r.InputTokens,
		OutputTokens:    r.OutputTokens,
		CostMicros:      r.CostMicros,
		LatencyMs:       r.LatencyMs,
		FirstTokenMs:    r.FirstTokenMs,
		PromptVersion:   r.PromptVersion,
		Flags:           orEmpty(r.Flags),
		Error:           runError(r),
		CreatedAt:       r.CreatedAt,
	}
}

/*
AiRunStep is one traced step of a run.
*/
type AiRunStep struct {
	ID           string          `json:"id"`
	Seq          int32           `json:"seq"`
	Kind         string          `json:"kind"`
	Provider     *string         `json:"provider"`
	Model        *string         `json:"model"`
	Status       string          `json:"status"`
	ErrorKind    *string         `json:"error_kind"`
	LatencyMs    *int32          `json:"latency_ms"`
	InputTokens  int64           `json:"input_tokens"`
	OutputTokens int64           `json:"output_tokens"`
	CostMicros   int64           `json:"cost_micros"`
	Request      json.RawMessage `json:"request"`
	Response     json.RawMessage `json:"response"`
}

/*
NewAiRunStep shapes a step for the API.
*/
func NewAiRunStep(s models.AiRunStep) AiRunStep {
	return AiRunStep{
		ID: models.PublicID(models.PrefixAiRunStep, s.ID), Seq: s.Seq, Kind: s.Kind,
		Provider: s.Provider, Model: s.Model, Status: s.Status, ErrorKind: s.ErrorKind,
		LatencyMs: s.LatencyMs, InputTokens: s.InputTokens, OutputTokens: s.OutputTokens,
		CostMicros: s.CostMicros, Request: s.Request, Response: s.Response,
	}
}

/*
runError pairs the run's failure fields.
*/
func runError(r models.AiRun) any {
	if r.ErrorCode == nil {
		return nil
	}
	return map[string]any{"code": r.ErrorCode, "message": r.ErrorMessage}
}

/*
publicPtr maps a nullable id to its public form.
*/
func publicPtr(prefix models.Prefix, id *ids.UUID) *string {
	if id == nil {
		return nil
	}
	s := models.PublicID(prefix, *id)
	return &s
}

