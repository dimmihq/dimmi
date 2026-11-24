/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Resources
|--------------------------------------------------------------------------
|
| An AI conversation attribute as the API shows it, and the values the
| decision model found on one conversation.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/aiattributes"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAttribute is one AI conversation attribute: its key, name, what it
means, its type, the choices of a choice, and whether it is active.
*/
type AIAttribute struct {
	ID          string    `json:"id"`
	Object      string    `json:"object"`
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	DataType    string    `json:"data_type"`
	Options     []string  `json:"options"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

/*
NewAIAttribute shapes an attribute.
*/
func NewAIAttribute(a models.AiConversationAttribute) AIAttribute {
	options := a.Options
	if options == nil {
		options = []string{}
	}

	return AIAttribute{
		ID: models.PublicID(models.PrefixAIAttribute, a.ID), Object: "ai_conversation_attribute", Key: a.Key,
		Label: a.Label, Description: a.Description, DataType: a.DataType, Options: options, Status: a.Status,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

/*
AIAttributeValue is one attribute of a conversation and the value
found for it, null while none was found.
*/
type AIAttributeValue struct {
	Object    string          `json:"object"`
	Attribute AIAttribute     `json:"attribute"`
	Value     json.RawMessage `json:"value"`
}

/*
ConversationAIAttributes is what was found on a conversation and when
it was last read.
*/
type ConversationAIAttributes struct {
	Object    string             `json:"object"`
	Values    []AIAttributeValue `json:"values"`
	CheckedAt *time.Time         `json:"checked_at"`
}

/*
NewConversationAIAttributes shapes what was found on a conversation.
*/
func NewConversationAIAttributes(f aiattributes.Found) ConversationAIAttributes {
	out := ConversationAIAttributes{
		Object: "conversation_ai_attributes", Values: []AIAttributeValue{}, CheckedAt: f.CheckedAt,
	}
	for _, v := range f.Values {
		value, err := json.Marshal(v.Value)
		if err != nil {
			value = json.RawMessage("null")
		}

		out.Values = append(out.Values, AIAttributeValue{
			Object: "ai_attribute_value", Attribute: NewAIAttribute(v.Attribute), Value: value,
		})
	}

	return out
}
