/*
|--------------------------------------------------------------------------
| Agent Definition
|--------------------------------------------------------------------------
|
| Everything a teammate edits on an agent, in one value. An agent's
| columns hold the published definition, which customers get; its draft
| holds the working copy until it is published. A version keeps a
| published definition as it was. Where the agent answers is part of the
| definition too, so placing an agent on a channel waits for publishing
| like every other change.
|
*/

package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Scope modes: an agent may use everything of a kind, or only what it
names.
*/
const (
	ScopeAll      = "all"
	ScopeSelected = "selected"
)

/*
Scope says what of one kind an agent may use: all of it, or only the
ids it names.
*/
type Scope struct {
	Mode string     `json:"mode"`
	IDs  []ids.UUID `json:"ids,omitempty"`
}

/*
KnowledgeScope says what an agent may search: everything, or only the
help centers and knowledge sources it names.
*/
type KnowledgeScope struct {
	Mode          string     `json:"mode"`
	SourceIDs     []ids.UUID `json:"source_ids,omitempty"`
	HelpCenterIDs []ids.UUID `json:"help_center_ids,omitempty"`
}

/*
Placement says where an agent takes new conversations: channel
connections (a widget, an email address, a number), Slack channels and
inboxes. Publishing writes it onto those rows.
*/
type Placement struct {
	ChannelConnectionIDs []ids.UUID `json:"channel_connection_ids,omitempty"`
	ChannelRoomIDs       []ids.UUID `json:"channel_room_ids,omitempty"`
	InboxIDs             []ids.UUID `json:"inbox_ids,omitempty"`
}

/*
Definition is an agent as a teammate edits it. Field names match the
API and the agent's columns.
*/
type Definition struct {
	Name                      string          `json:"name"`
	AvatarAttachmentID        *ids.UUID       `json:"avatar_attachment_id,omitempty"`
	ProductName               *string         `json:"product_name,omitempty"`
	Tone                      string          `json:"tone"`
	Guidance                  string          `json:"guidance"`
	EnabledChannels           []string        `json:"enabled_channels"`
	ExcludedConnectionIDs     []ids.UUID      `json:"excluded_channel_connection_ids,omitempty"`
	Schedule                  string          `json:"schedule"`
	AssignedBehavior          string          `json:"assigned_behavior"`
	AnswerKeyID               *ids.UUID       `json:"answer_key_id,omitempty"`
	AnswerModel               *string         `json:"answer_model,omitempty"`
	FallbackKeyID             *ids.UUID       `json:"fallback_key_id,omitempty"`
	FallbackModel             *string         `json:"fallback_model,omitempty"`
	MaxAiReplies              int32           `json:"max_ai_replies"`
	UnknownBehavior           string          `json:"unknown_behavior"`
	RequireCitations          bool            `json:"require_citations"`
	Languages                 []string        `json:"languages"`
	DefaultLanguage           string          `json:"default_language"`
	Streaming                 bool            `json:"streaming"`
	IncludeContactContext     string          `json:"include_contact_context"`
	ContactAttributeAllowlist []string        `json:"contact_attribute_allowlist"`
	PiiRedaction              bool            `json:"pii_redaction"`
	LinkAllowlist             []string        `json:"link_allowlist"`
	Handoff                   json.RawMessage `json:"handoff"`
	Messages                  json.RawMessage `json:"messages"`
	ActionsEnabled            bool            `json:"actions_enabled"`
	Knowledge                 KnowledgeScope  `json:"knowledge_scope"`
	Procedures                Scope           `json:"procedure_scope"`
	Actions                   Scope           `json:"action_scope"`
	TransferAgentIDs          []ids.UUID      `json:"transfer_agent_ids"`
	Placement                 Placement       `json:"placement"`
}

/*
Published returns the definition customers get: the agent's columns,
with where it answers as the caller read it from the channel rows.
*/
func Published(agent models.AiAgent, placement Placement) Definition {
	return Definition{
		Name: agent.Name, AvatarAttachmentID: agent.AvatarAttachmentID, ProductName: agent.ProductName,
		Tone: agent.Tone, Guidance: agent.Guidance,
		EnabledChannels: agent.EnabledChannels, ExcludedConnectionIDs: agent.ExcludedChannelConnectionIds,
		Schedule: agent.Schedule, AssignedBehavior: agent.AssignedBehavior,
		AnswerKeyID: agent.AnswerKeyID, AnswerModel: agent.AnswerModel,
		FallbackKeyID: agent.FallbackKeyID, FallbackModel: agent.FallbackModel,
		MaxAiReplies: agent.MaxAiReplies, UnknownBehavior: agent.UnknownBehavior,
		RequireCitations: agent.RequireCitations, Languages: agent.Languages,
		DefaultLanguage: agent.DefaultLanguage, Streaming: agent.Streaming,
		IncludeContactContext: agent.IncludeContactContext, ContactAttributeAllowlist: agent.ContactAttributeAllowlist,
		PiiRedaction: agent.PiiRedaction, LinkAllowlist: agent.LinkAllowlist,
		Handoff: agent.Handoff, Messages: agent.Messages,
		ActionsEnabled: agent.ActionsEnabled,
		Knowledge:      KnowledgeOf(agent), Procedures: ProceduresOf(agent), Actions: ActionsOf(agent),
		TransferAgentIDs: agent.TransferAgentIds, Placement: placement,
	}
}

/*
Working returns the definition a teammate is editing: the draft when
there is one, otherwise the published definition.
*/
func Working(agent models.AiAgent, placement Placement) (Definition, error) {
	if len(agent.Draft) == 0 || string(agent.Draft) == "null" {
		return Published(agent, placement), nil
	}
	// A draft holds the whole definition, so a field it leaves out was
	// cleared, not kept from the published copy.
	var def Definition
	if err := json.Unmarshal(agent.Draft, &def); err != nil {
		return Definition{}, fmt.Errorf("read agent draft: %w", err)
	}
	return def, nil
}

/*
Apply writes a definition onto an agent row's columns, as publishing
does, or as a simulation answering as the draft does in memory.
*/
func Apply(agent *models.AiAgent, def Definition) {
	agent.Name = def.Name
	agent.AvatarAttachmentID = def.AvatarAttachmentID
	agent.ProductName = def.ProductName
	agent.Tone = def.Tone
	agent.Guidance = def.Guidance
	agent.EnabledChannels = nonNil(def.EnabledChannels)
	agent.ExcludedChannelConnectionIds = nonNilIDs(def.ExcludedConnectionIDs)
	agent.Schedule = def.Schedule
	agent.AssignedBehavior = def.AssignedBehavior
	agent.AnswerKeyID = def.AnswerKeyID
	agent.AnswerModel = def.AnswerModel
	agent.FallbackKeyID = def.FallbackKeyID
	agent.FallbackModel = def.FallbackModel
	agent.MaxAiReplies = def.MaxAiReplies
	agent.UnknownBehavior = def.UnknownBehavior
	agent.RequireCitations = def.RequireCitations
	agent.Languages = nonNil(def.Languages)
	agent.DefaultLanguage = def.DefaultLanguage
	agent.Streaming = def.Streaming
	agent.IncludeContactContext = def.IncludeContactContext
	agent.ContactAttributeAllowlist = nonNil(def.ContactAttributeAllowlist)
	agent.PiiRedaction = def.PiiRedaction
	agent.LinkAllowlist = nonNil(def.LinkAllowlist)
	agent.Handoff = rawOr(def.Handoff, "{}")
	agent.Messages = rawOr(def.Messages, "{}")
	agent.ActionsEnabled = def.ActionsEnabled
	agent.KnowledgeScope = asJSON(normalKnowledge(def.Knowledge))
	agent.ProcedureScope = asJSON(normalScope(def.Procedures))
	agent.ActionScope = asJSON(normalScope(def.Actions))
	agent.TransferAgentIds = nonNilIDs(def.TransferAgentIDs)
}

/*
Same reports whether two definitions say the same thing, so a draft
edited back to what is published is dropped.
*/
func Same(a, b Definition) bool {
	return bytes.Equal(canonical(a), canonical(b))
}

/*
canonical encodes a definition with its stored JSON fields re-encoded,
so two definitions that differ only in spacing or key order match.
*/
func canonical(d Definition) []byte {
	var v any
	if json.Unmarshal(asJSON(d), &v) != nil {
		return asJSON(d)
	}
	return asJSON(v)
}

/*
UpdateParams turns an agent row into the parameters that save its
published columns.
*/
func UpdateParams(agent *models.AiAgent, now time.Time) models.UpdateAgentParams {
	return models.UpdateAgentParams{
		Name: agent.Name, AvatarAttachmentID: agent.AvatarAttachmentID, Enabled: agent.Enabled,
		ProductName: agent.ProductName, Tone: agent.Tone, Guidance: agent.Guidance,
		EnabledChannels: agent.EnabledChannels, ExcludedChannelConnectionIds: agent.ExcludedChannelConnectionIds,
		Schedule: agent.Schedule, AssignedBehavior: agent.AssignedBehavior,
		AnswerKeyID: agent.AnswerKeyID, AnswerModel: agent.AnswerModel,
		ClassifierKeyID: agent.ClassifierKeyID, ClassifierModel: agent.ClassifierModel,
		FallbackKeyID: agent.FallbackKeyID, FallbackModel: agent.FallbackModel,
		Temperature: agent.Temperature, MaxOutputTokens: agent.MaxOutputTokens, MaxAiReplies: agent.MaxAiReplies,
		UnknownBehavior: agent.UnknownBehavior, GroundingMode: agent.GroundingMode,
		RequireCitations: agent.RequireCitations, MinSimilarity: agent.MinSimilarity,
		Languages: agent.Languages, DefaultLanguage: agent.DefaultLanguage, Streaming: agent.Streaming,
		IncludeContactContext: agent.IncludeContactContext, ContactAttributeAllowlist: agent.ContactAttributeAllowlist,
		PiiRedaction: agent.PiiRedaction, LinkAllowlist: agent.LinkAllowlist,
		Handoff: agent.Handoff, Messages: agent.Messages, PromptVersion: agent.PromptVersion,
		ActionsEnabled: agent.ActionsEnabled,
		KnowledgeScope: agent.KnowledgeScope, ProcedureScope: agent.ProcedureScope, ActionScope: agent.ActionScope,
		TransferAgentIds: agent.TransferAgentIds,
		Now:              now, WorkspaceID: agent.WorkspaceID, ID: agent.ID,
	}
}

/*
KnowledgeOf reads what the agent may search from its published
columns. A missing or unreadable scope means everything.
*/
func KnowledgeOf(agent models.AiAgent) KnowledgeScope {
	var s KnowledgeScope
	if json.Unmarshal(agent.KnowledgeScope, &s) != nil {
		return KnowledgeScope{Mode: ScopeAll}
	}
	return normalKnowledge(s)
}

/*
ProceduresOf reads which procedures the agent may follow.
*/
func ProceduresOf(agent models.AiAgent) Scope {
	return scopeOf(agent.ProcedureScope)
}

/*
ActionsOf reads which actions the agent may call.
*/
func ActionsOf(agent models.AiAgent) Scope {
	return scopeOf(agent.ActionScope)
}

/*
Allows reports whether a scope lets the agent use one id.
*/
func (s Scope) Allows(id ids.UUID) bool {
	return s.Mode != ScopeSelected || slices.Contains(s.IDs, id)
}

func scopeOf(raw json.RawMessage) Scope {
	var s Scope
	if json.Unmarshal(raw, &s) != nil {
		return Scope{Mode: ScopeAll}
	}
	return normalScope(s)
}

func normalScope(s Scope) Scope {
	if s.Mode != ScopeSelected {
		return Scope{Mode: ScopeAll}
	}
	return Scope{Mode: ScopeSelected, IDs: nonNilIDs(s.IDs)}
}

func normalKnowledge(s KnowledgeScope) KnowledgeScope {
	if s.Mode != ScopeSelected {
		return KnowledgeScope{Mode: ScopeAll}
	}
	return KnowledgeScope{
		Mode: ScopeSelected, SourceIDs: nonNilIDs(s.SourceIDs), HelpCenterIDs: nonNilIDs(s.HelpCenterIDs),
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilIDs(s []ids.UUID) []ids.UUID {
	if s == nil {
		return []ids.UUID{}
	}
	return s
}

func rawOr(raw json.RawMessage, empty string) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(empty)
	}
	return raw
}

/*
asJSON encodes a value the package defines, which always encodes.
*/
func asJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
