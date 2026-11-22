/*
|--------------------------------------------------------------------------
| AI Agent Version Resources
|--------------------------------------------------------------------------
|
| A published version of an agent, the templates a new agent starts
| from, and the workspace's shared AI settings.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAgentVersion is one published version of an agent: its number, the
definition as published, who published it and why.
*/
type AIAgentVersion struct {
	ID                  string            `json:"id"`
	Object              string            `json:"object"`
	AgentID             string            `json:"agent_id"`
	Version             int32             `json:"version"`
	Definition          AIAgentDefinition `json:"definition"`
	ChangeNote          *string           `json:"change_note"`
	PublishedByMemberID *string           `json:"published_by_member_id"`
	PublishedAt         time.Time         `json:"published_at"`
}

/*
NewAIAgentVersion shapes a version for the API.
*/
func NewAIAgentVersion(v ai.AgentVersion) AIAgentVersion {
	return AIAgentVersion{
		ID: models.PublicID(models.PrefixAiAgentVersion, v.Version.ID), Object: "ai_agent_version",
		AgentID: models.PublicID(models.PrefixAiAgent, v.Version.AgentID), Version: v.Version.Version,
		Definition: NewAIAgentDefinition(v.Definition), ChangeNote: v.Version.ChangeNote,
		PublishedByMemberID: publicPtr(models.PrefixMember, v.Version.PublishedByMemberID),
		PublishedAt:         v.Version.PublishedAt,
	}
}

/*
AIAgentTemplate is a starting point for a new agent: its key (empty for
starting from nothing), the group the picker lists it under, how the
picker shows it, the name it suggests, the agent's role, and whether
it usually lives on a page of its own.
*/
type AIAgentTemplate struct {
	Key         string `json:"key"`
	Object      string `json:"object"`
	Group       string `json:"group"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AgentName   string `json:"agent_name"`
	Role        string `json:"role"`
	Widget      bool   `json:"widget"`
}

/*
NewAIAgentTemplates shapes the templates for the API.
*/
func NewAIAgentTemplates(list []ai.Template) []AIAgentTemplate {
	out := make([]AIAgentTemplate, len(list))
	for i, t := range list {
		out[i] = AIAgentTemplate{
			Key: t.Key, Object: "ai_agent_template", Group: string(t.Group), Name: t.Name,
			Description: t.Description, AgentName: t.AgentName, Role: t.Role, Widget: t.Widget,
		}
	}
	return out
}

/*
AISettings is what every agent and Operator share: the answer and
fallback models an agent uses until it picks its own, the classifier,
and the monthly spend cap, null when there is none.
*/
type AISettings struct {
	Object                string  `json:"object"`
	AnswerKeyID           *string `json:"answer_key_id"`
	AnswerModel           *string `json:"answer_model"`
	ClassifierKeyID       *string `json:"classifier_key_id"`
	ClassifierModel       *string `json:"classifier_model"`
	FallbackKeyID         *string `json:"fallback_key_id"`
	FallbackModel         *string `json:"fallback_model"`
	MonthlySpendCapMicros *int64  `json:"monthly_spend_cap_micros"`
}

/*
NewAISettings shapes the workspace's AI settings for the API.
*/
func NewAISettings(s models.WorkspaceAiSetting) AISettings {
	return AISettings{
		Object:      "ai_settings",
		AnswerKeyID: publicPtr(models.PrefixAiProviderKey, s.AnswerKeyID), AnswerModel: s.AnswerModel,
		ClassifierKeyID: publicPtr(models.PrefixAiProviderKey, s.ClassifierKeyID), ClassifierModel: s.ClassifierModel,
		FallbackKeyID: publicPtr(models.PrefixAiProviderKey, s.FallbackKeyID), FallbackModel: s.FallbackModel,
		MonthlySpendCapMicros: s.MonthlySpendCapMicros,
	}
}
