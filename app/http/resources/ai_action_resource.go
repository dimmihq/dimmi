/*
|--------------------------------------------------------------------------
| Agent Action Resources
|--------------------------------------------------------------------------
|
| Connections and actions as the dashboard sees them. A connection
| never shows its secret, only whether it has one and its last four
| characters. An action shows its working copy, whether that differs
| from the live version, and which version is live. Every id is a
| public one.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
)

/*
AIActionConnection is one connection, its secret withheld.
*/
type AIActionConnection struct {
	ID             string                   `json:"id"`
	Object         string                   `json:"object"`
	Name           string                   `json:"name"`
	Kind           string                   `json:"kind"`
	BaseURL        string                   `json:"base_url"`
	AuthType       string                   `json:"auth_type"`
	AuthConfig     actionkit.ConnectionAuth `json:"auth_config"`
	HasSecret      bool                     `json:"has_secret"`
	SecretHint     *string                  `json:"secret_hint"`
	DefaultHeaders map[string]string        `json:"default_headers"`
	Variables      map[string]string        `json:"variables"`
	TimeoutMS      int32                    `json:"timeout_ms"`
	Status         string                   `json:"status"`
	StatusReason   *string                  `json:"status_reason"`
	LastError      *string                  `json:"last_error"`
	LastErrorAt    *time.Time               `json:"last_error_at"`
	LastSuccessAt  *time.Time               `json:"last_success_at"`
	ToolsSyncedAt  *time.Time               `json:"tools_synced_at"`
	ActionsCount   int64                    `json:"actions_count"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

/*
NewAIActionConnection shapes a connection for the API.
*/
func NewAIActionConnection(c models.AiActionConnection) AIActionConnection {
	out := AIActionConnection{
		ID: models.PublicID(models.PrefixAiActionConnection, c.ID), Object: "ai_action_connection",
		Name: c.Name, Kind: c.Kind, BaseURL: c.BaseUrl, AuthType: c.AuthType,
		HasSecret: len(c.CredentialsEncrypted) > 0, SecretHint: c.SecretHint, TimeoutMS: c.TimeoutMs,
		Status: c.Status, StatusReason: c.StatusReason, LastError: c.LastError, LastErrorAt: c.LastErrorAt,
		LastSuccessAt: c.LastSuccessAt, ToolsSyncedAt: c.ToolsSyncedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		DefaultHeaders: map[string]string{}, Variables: map[string]string{},
	}
	_ = json.Unmarshal(c.AuthConfig, &out.AuthConfig)
	_ = json.Unmarshal(c.DefaultHeaders, &out.DefaultHeaders)
	_ = json.Unmarshal(c.Variables, &out.Variables)
	return out
}

/*
NewAIActionConnections shapes a list of connections with how many
actions use each.
*/
func NewAIActionConnections(rows []models.AiActionConnection, counts map[ids.UUID]int64) []AIActionConnection {
	out := make([]AIActionConnection, len(rows))
	for i, c := range rows {
		out[i] = NewAIActionConnection(c)
		out[i].ActionsCount = counts[c.ID]
	}
	return out
}

/*
AIActionVersionRef names a published version.
*/
type AIActionVersionRef struct {
	ID        string    `json:"id"`
	Number    int32     `json:"number"`
	CreatedAt time.Time `json:"created_at"`
}

/*
AIAction is one action's working copy. HasUnpublishedChanges says the
working copy differs from the live version; PublishedVersion is null
until the first publication.
*/
type AIAction struct {
	ID                     string                       `json:"id"`
	Object                 string                       `json:"object"`
	ConnectionID           string                       `json:"connection_id"`
	Name                   string                       `json:"name"`
	Title                  string                       `json:"title"`
	Description            string                       `json:"description"`
	Kind                   string                       `json:"kind"`
	SideEffect             string                       `json:"side_effect"`
	Confirmation           string                       `json:"confirmation"`
	Approval               string                       `json:"approval"`
	ApprovalTimeoutMinutes int32                        `json:"approval_timeout_minutes"`
	Status                 string                       `json:"status"`
	Method                 *string                      `json:"method"`
	PathTemplate           *string                      `json:"path_template"`
	Query                  map[string]string            `json:"query"`
	Headers                map[string]string            `json:"headers"`
	BodyTemplate           json.RawMessage              `json:"body_template"`
	BodyEncoding           string                       `json:"body_encoding"`
	MCPToolName            *string                      `json:"mcp_tool_name"`
	BoundArguments         map[string]any               `json:"bound_arguments"`
	InputSchema            json.RawMessage              `json:"input_schema"`
	ResponseFields         []actionresult.ResponseField `json:"response_fields"`
	ErrorMessagePath       *string                      `json:"error_message_path"`
	ResponseMaxBytes       int32                        `json:"response_max_bytes"`
	TimeoutMS              *int32                       `json:"timeout_ms"`
	Channels               []string                     `json:"channels"`
	Agents                 []string                     `json:"agents"`
	Source                 string                       `json:"source"`
	Warnings               []string                     `json:"warnings"`
	HasUnpublishedChanges  bool                         `json:"has_unpublished_changes"`
	PublishedVersion       *AIActionVersionRef          `json:"published_version"`
	CreatedAt              time.Time                    `json:"created_at"`
	UpdatedAt              time.Time                    `json:"updated_at"`
}

/*
NewAIAction shapes an action and its live version for the API.
*/
func NewAIAction(v agentactions.View) AIAction {
	a := v.Action
	s := actionkit.FromAction(a)
	out := AIAction{
		ID: models.PublicID(models.PrefixAiAction, a.ID), Object: "ai_action",
		ConnectionID: models.PublicID(models.PrefixAiActionConnection, a.ConnectionID),
		Name:         a.Name, Title: a.Title, Description: a.Description, Kind: a.Kind, SideEffect: a.SideEffect,
		Confirmation: a.Confirmation, Approval: a.Approval, ApprovalTimeoutMinutes: a.ApprovalTimeoutMinutes,
		Status: a.Status, Method: a.Method, PathTemplate: a.PathTemplate, Query: s.Query, Headers: s.Headers,
		BodyTemplate: a.BodyTemplate, BodyEncoding: a.BodyEncoding, MCPToolName: a.McpToolName,
		BoundArguments: s.BoundArguments, InputSchema: a.InputSchema, ResponseFields: orEmpty(s.ResponseFields),
		ErrorMessagePath: a.ErrorMessagePath, ResponseMaxBytes: a.ResponseMaxBytes, TimeoutMS: a.TimeoutMs,
		Channels: orEmpty(a.Channels), Agents: orEmpty(a.Agents), Source: a.Source, Warnings: []string{},
		HasUnpublishedChanges: true,
		CreatedAt:             a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if v.Version != nil {
		out.PublishedVersion = &AIActionVersionRef{
			ID:     models.PublicID(models.PrefixAiActionVersion, v.Version.ID),
			Number: v.Version.Number, CreatedAt: v.Version.CreatedAt,
		}
		out.HasUnpublishedChanges = v.Version.SpecHash != a.SpecHash
	}
	return out
}

/*
NewAIActions shapes a list of actions.
*/
func NewAIActions(rows []agentactions.View) []AIAction {
	out := make([]AIAction, len(rows))
	for i, v := range rows {
		out[i] = NewAIAction(v)
	}
	return out
}

/*
AIActionVersion is one published version, the definition it froze and
who published it.
*/
type AIActionVersion struct {
	ID          string           `json:"id"`
	Object      string           `json:"object"`
	ActionID    string           `json:"action_id"`
	Number      int32            `json:"number"`
	Spec        json.RawMessage  `json:"spec"`
	Note        *string          `json:"note"`
	PublishedBy *MessageAIPerson `json:"published_by"`
	CreatedAt   time.Time        `json:"created_at"`
}

/*
NewAIActionVersion shapes a version for the API with the publisher's
name, when known.
*/
func NewAIActionVersion(v models.AiActionVersion, names map[ids.UUID]string) AIActionVersion {
	out := AIActionVersion{
		ID: models.PublicID(models.PrefixAiActionVersion, v.ID), Object: "ai_action_version",
		ActionID: models.PublicID(models.PrefixAiAction, v.ActionID), Number: v.Number, Spec: v.Spec,
		Note: v.Note, CreatedAt: v.CreatedAt,
	}
	if v.PublishedByMemberID != nil {
		out.PublishedBy = &MessageAIPerson{
			ID: models.PublicID(models.PrefixMember, *v.PublishedByMemberID), Name: names[*v.PublishedByMemberID],
		}
	}
	return out
}
