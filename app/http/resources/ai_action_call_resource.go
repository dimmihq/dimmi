/*
|--------------------------------------------------------------------------
| Agent Action Call Resources
|--------------------------------------------------------------------------
|
| A call as the team sees it: where it stands, who had to agree and who
| did, and what came back. Its input, request, response and result are
| payloads, included only for those who may read payloads. The visitor
| sees far less: the title, the summary they confirm, where it stands
| and when it lapses.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIActionCall is one call. Input, Request, Response and Result are
present only with payload access.
*/
type AIActionCall struct {
	ID                string          `json:"id"`
	Object            string          `json:"object"`
	ActionID          string          `json:"action_id"`
	ActionVersionID   *string         `json:"action_version_id"`
	ConnectionID      string          `json:"connection_id"`
	ConversationID    *string         `json:"conversation_id"`
	RunID             *string         `json:"run_id"`
	MessageID         *string         `json:"message_id"`
	Mode              string          `json:"mode"`
	SideEffect        string          `json:"side_effect"`
	Confirmation      string          `json:"confirmation"`
	Approval          string          `json:"approval"`
	State             string          `json:"state"`
	Title             string          `json:"title"`
	Summary           string          `json:"summary"`
	ErrorCode         *string         `json:"error_code"`
	HTTPStatus        *int32          `json:"http_status"`
	DecidedByMemberID *string         `json:"decided_by_member_id"`
	DecisionReason    *string         `json:"decision_reason"`
	CustomerMessage   *string         `json:"customer_message"`
	CancelReason      *string         `json:"cancel_reason"`
	LatencyMS         *int32          `json:"latency_ms"`
	RequestedAt       time.Time       `json:"requested_at"`
	ConfirmedAt       *time.Time      `json:"confirmed_at"`
	DecidedAt         *time.Time      `json:"decided_at"`
	SentAt            *time.Time      `json:"sent_at"`
	CompletedAt       *time.Time      `json:"completed_at"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	ReportedAt        *time.Time      `json:"reported_at"`
	Input             json.RawMessage `json:"input,omitempty"`
	Request           json.RawMessage `json:"request,omitempty"`
	Response          json.RawMessage `json:"response,omitempty"`
	Result            json.RawMessage `json:"result,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

/*
NewAIActionCall shapes a call without its payloads.
*/
func NewAIActionCall(c models.AiActionCall) AIActionCall {
	out := AIActionCall{
		ID: models.PublicID(models.PrefixAiActionCall, c.ID), Object: "ai_action_call",
		ActionID:        models.PublicID(models.PrefixAiAction, c.ActionID),
		ActionVersionID: publicPtr(models.PrefixAiActionVersion, c.ActionVersionID),
		ConnectionID:    models.PublicID(models.PrefixAiActionConnection, c.ConnectionID),
		ConversationID:  publicPtr(models.PrefixConversation, c.ConversationID),
		RunID:           publicPtr(models.PrefixAiRun, c.RunID),
		MessageID:       publicPtr(models.PrefixMessage, c.MessageID),
		Mode:            c.Mode, SideEffect: c.SideEffect, Confirmation: c.Confirmation, Approval: c.Approval,
		State: c.State, Title: c.Title, Summary: c.Summary, ErrorCode: c.ErrorCode, HTTPStatus: c.HttpStatus,
		DecidedByMemberID: publicPtr(models.PrefixMember, c.DecidedByMemberID), DecisionReason: c.DecisionReason,
		CustomerMessage: c.CustomerMessage, CancelReason: c.CancelReason, LatencyMS: c.LatencyMs,
		RequestedAt: c.RequestedAt, ConfirmedAt: c.ConfirmedAt, DecidedAt: c.DecidedAt, SentAt: c.SentAt,
		CompletedAt: c.CompletedAt, ExpiresAt: c.ExpiresAt, ReportedAt: c.ReportedAt,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	return out
}

/*
WithPayloads adds the call's input, request, response and result, for
a caller who may read payloads.
*/
func (r AIActionCall) WithPayloads(c models.AiActionCall) AIActionCall {
	r.Input, r.Request, r.Response, r.Result = c.Input, c.Request, c.Response, c.Result
	return r
}

/*
NewAIActionCalls shapes a list of calls, with their payloads for a
caller who may read them.
*/
func NewAIActionCalls(rows []models.AiActionCall, m authz.Membership) []AIActionCall {
	out := make([]AIActionCall, len(rows))
	for i, c := range rows {
		out[i] = ShowAIActionCall(c, m)
	}
	return out
}

/*
ShowAIActionCall shapes a call, with its payloads for a caller who may
read them.
*/
func ShowAIActionCall(c models.AiActionCall, m authz.Membership) AIActionCall {
	out := NewAIActionCall(c)
	if agentactions.CanReadPayloads(m) {
		return out.WithPayloads(c)
	}
	return out
}

/*
WidgetActionCall is a call as the visitor sees it on the confirmation
card. Status repeats State, the name the card reads.
*/
type WidgetActionCall struct {
	ID        string     `json:"id"`
	Object    string     `json:"object"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	State     string     `json:"state"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

/*
NewWidgetActionCall shapes a call for the visitor.
*/
func NewWidgetActionCall(c models.AiActionCall) WidgetActionCall {
	return WidgetActionCall{
		ID: models.PublicID(models.PrefixAiActionCall, c.ID), Object: "action_call", Title: c.Title,
		Summary: c.Summary, State: c.State, Status: c.State, ExpiresAt: c.ExpiresAt,
	}
}

/*
AIActionTest is how a test of an action went.
*/
type AIActionTest struct {
	Object           string          `json:"object"`
	Status           string          `json:"status"`
	ErrorCode        *string         `json:"error_code"`
	ValidationErrors []string        `json:"validation_errors"`
	Request          json.RawMessage `json:"request"`
	Response         json.RawMessage `json:"response"`
	Result           json.RawMessage `json:"result"`
	Summary          string          `json:"summary"`
	ConfirmationText string          `json:"confirmation_text"`
	Warnings         []string        `json:"warnings"`
	LatencyMS        int64           `json:"latency_ms"`
	CallID           *string         `json:"call_id"`
}

/*
NewAIActionTest shapes a test result.
*/
func NewAIActionTest(r agentactions.TestResult) AIActionTest {
	out := AIActionTest{
		Object: "ai_action_test", Status: r.Status, ValidationErrors: orEmpty(r.ValidationErrors),
		Request: r.Request, Response: r.Response, Result: r.Result, Summary: r.Summary,
		ConfirmationText: r.Summary, Warnings: []string{},
		LatencyMS: r.Latency.Milliseconds(), CallID: publicPtr(models.PrefixAiActionCall, r.CallID),
	}
	if r.ErrorCode != "" {
		out.ErrorCode = &r.ErrorCode
	}
	return out
}

/*
AIActionConnectionTest is how a connection probe went.
*/
type AIActionConnectionTest struct {
	Object       string  `json:"object"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason"`
	HTTPStatus   *int    `json:"http_status"`
	LatencyMS    int64   `json:"latency_ms"`
	ToolsCount   *int    `json:"tools_count"`
}

/*
NewAIActionConnectionTest shapes a probe result.
*/
func NewAIActionConnectionTest(kind string, r actionkit.ProbeResult) AIActionConnectionTest {
	out := AIActionConnectionTest{
		Object: "ai_action_connection_test", Status: r.Status, LatencyMS: r.Latency.Milliseconds(),
	}
	if r.Reason != "" {
		out.StatusReason = &r.Reason
	}
	if r.HTTPStatus != 0 {
		out.HTTPStatus = &r.HTTPStatus
	}
	if kind == "mcp" {
		out.ToolsCount = &r.ToolsCount
	}
	return out
}

/*
AIActionToolSync is what an MCP tool sync did. A sync never changes or
disables an action it made before, so Updated and Disabled stay empty
and Existing counts the tools that already have one.
*/
type AIActionToolSync struct {
	Object   string                     `json:"object"`
	Created  []AIAction                 `json:"created"`
	Updated  []AIAction                 `json:"updated"`
	Disabled []AIAction                 `json:"disabled"`
	Existing int                        `json:"existing"`
	Skipped  []agentactions.SkippedTool `json:"skipped"`
}

/*
NewAIActionToolSync shapes a sync result.
*/
func NewAIActionToolSync(s agentactions.ToolSync) AIActionToolSync {
	created := make([]AIAction, len(s.Created))
	for i, a := range s.Created {
		created[i] = NewAIAction(agentactions.View{Action: a})
	}
	return AIActionToolSync{
		Object: "ai_action_tool_sync", Created: created, Updated: []AIAction{}, Disabled: []AIAction{},
		Existing: s.Existing, Skipped: orEmpty(s.Skipped),
	}
}
