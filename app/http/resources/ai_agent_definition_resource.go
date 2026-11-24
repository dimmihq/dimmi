/*
|--------------------------------------------------------------------------
| AI Agent Definition Resources
|--------------------------------------------------------------------------
|
| Everything a teammate edits on an agent, as the API reads it: who it
| is, how it answers, what it may use and where it answers.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIScope is what an agent may use of one kind: mode "all", or "selected"
with the public ids it names.
*/
type AIScope struct {
	Mode string   `json:"mode"`
	IDs  []string `json:"ids"`
}

/*
AIKnowledgeScope is what an agent may search: mode "all", or
"selected" with the help centers and knowledge sources it names.
*/
type AIKnowledgeScope struct {
	Mode          string   `json:"mode"`
	HelpCenterIDs []string `json:"help_center_ids"`
	SourceIDs     []string `json:"source_ids"`
}

/*
AIPlacement is where an agent takes new conversations: channel
connections, Slack channels and inboxes.
*/
type AIPlacement struct {
	ChannelConnectionIDs []string `json:"channel_connection_ids"`
	ChannelRoomIDs       []string `json:"channel_room_ids"`
	InboxIDs             []string `json:"inbox_ids"`
}

/*
AIAgentDefinition is everything a teammate edits on an agent.
Connection, team, tag and agent ids are public ids. HandoffMode is full
or triage_only; AlwaysHandoffTagIDs are the tags that always send a
conversation to the team.
*/
type AIAgentDefinition struct {
	Name                  string           `json:"name"`
	ProductName           *string          `json:"product_name"`
	Tone                  string           `json:"tone"`
	Guidance              string           `json:"guidance"`
	Description           string           `json:"description"`
	EnabledChannels       []string         `json:"enabled_channels"`
	Schedule              string           `json:"schedule"`
	AssignedBehavior      string           `json:"assigned_behavior"`
	AnswerKeyID           *string          `json:"answer_key_id"`
	AnswerModel           *string          `json:"answer_model"`
	FallbackKeyID         *string          `json:"fallback_key_id"`
	FallbackModel         *string          `json:"fallback_model"`
	MaxAiReplies          int              `json:"max_ai_replies"`
	UnknownBehavior       string           `json:"unknown_behavior"`
	RequireCitations      bool             `json:"require_citations"`
	Languages             []string         `json:"languages"`
	DefaultLanguage       string           `json:"default_language"`
	Streaming             bool             `json:"streaming"`
	IncludeContactContext string           `json:"include_contact_context"`
	PiiRedaction          bool             `json:"pii_redaction"`
	LinkAllowlist         []string         `json:"link_allowlist"`
	HandoffTeamID         *string          `json:"handoff_team_id"`
	RaisePriority         bool             `json:"raise_priority_on_handoff"`
	HandoffMode           string           `json:"handoff_mode"`
	AlwaysHandoffTagIDs   []string         `json:"always_handoff_tag_ids"`
	KnowledgeScope        AIKnowledgeScope `json:"knowledge_scope"`
	ProcedureScope        AIScope          `json:"procedure_scope"`
	ActionScope           AIScope          `json:"action_scope"`
	TransferAgentIDs      []string         `json:"transfer_agent_ids"`
	Placement             AIPlacement      `json:"placement"`
}

/*
NewAIAgentDefinition shapes a definition for the API.
*/
func NewAIAgentDefinition(d agents.Definition) AIAgentDefinition {
	handoff := ai.ReadHandoff(d.Handoff)
	return AIAgentDefinition{
		Name: d.Name, ProductName: d.ProductName, Tone: d.Tone, Guidance: d.Guidance, Description: d.Description,
		EnabledChannels: orEmpty(d.EnabledChannels), Schedule: d.Schedule, AssignedBehavior: d.AssignedBehavior,
		AnswerKeyID: publicPtr(models.PrefixAiProviderKey, d.AnswerKeyID), AnswerModel: d.AnswerModel,
		FallbackKeyID: publicPtr(models.PrefixAiProviderKey, d.FallbackKeyID), FallbackModel: d.FallbackModel,
		MaxAiReplies: int(d.MaxAiReplies), UnknownBehavior: d.UnknownBehavior, RequireCitations: d.RequireCitations,
		Languages: orEmpty(d.Languages), DefaultLanguage: d.DefaultLanguage, Streaming: d.Streaming,
		IncludeContactContext: d.IncludeContactContext, PiiRedaction: d.PiiRedaction,
		LinkAllowlist: orEmpty(d.LinkAllowlist),
		HandoffTeamID: publicPtr(models.PrefixTeam, handoff.TargetTeamID), RaisePriority: handoff.RaisePriority,
		HandoffMode: string(handoff.Mode), AlwaysHandoffTagIDs: publicIDs(models.PrefixTag, handoff.AlwaysTags),
		KnowledgeScope: AIKnowledgeScope{
			Mode:          d.Knowledge.Mode,
			HelpCenterIDs: publicIDs(models.PrefixHelpCenter, d.Knowledge.HelpCenterIDs),
			SourceIDs:     publicIDs(models.PrefixKnowledgeSource, d.Knowledge.SourceIDs),
		},
		ProcedureScope:   AIScope{Mode: d.Procedures.Mode, IDs: publicIDs(models.PrefixProcedure, d.Procedures.IDs)},
		ActionScope:      AIScope{Mode: d.Actions.Mode, IDs: publicIDs(models.PrefixAiAction, d.Actions.IDs)},
		TransferAgentIDs: publicIDs(models.PrefixAiAgent, d.TransferAgentIDs),
		Placement: AIPlacement{
			ChannelConnectionIDs: publicIDs(models.PrefixChannelConnection, d.Placement.ChannelConnectionIDs),
			ChannelRoomIDs:       publicIDs(models.PrefixChannelRoom, d.Placement.ChannelRoomIDs),
			InboxIDs:             publicIDs(models.PrefixInbox, d.Placement.InboxIDs),
		},
	}
}
