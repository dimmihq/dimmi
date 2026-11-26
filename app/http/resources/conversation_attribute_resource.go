/*
|--------------------------------------------------------------------------
| Conversation Attribute Resources
|--------------------------------------------------------------------------
|
| A conversation attribute as the API shows it, and the values of one
| conversation with who set each.
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
ConversationAttribute is one conversation attribute: its key, name,
what it means, its type, the choices of a list, who fills it, whether
it is required to close, the teams that see it, and whether it is
active.
*/
type ConversationAttribute struct {
	ID              string    `json:"id"`
	Object          string    `json:"object"`
	Key             string    `json:"key"`
	Label           string    `json:"label"`
	Description     string    `json:"description"`
	DataType        string    `json:"data_type"`
	Options         []string  `json:"options"`
	FilledBy        string    `json:"filled_by"`
	RequiredToClose bool      `json:"required_to_close"`
	TeamIDs         []string  `json:"team_ids"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

/*
NewConversationAttribute shapes an attribute.
*/
func NewConversationAttribute(a models.AiConversationAttribute) ConversationAttribute {
	options := a.Options
	if options == nil {
		options = []string{}
	}

	teams := make([]string, 0, len(a.TeamIds))
	for _, id := range a.TeamIds {
		teams = append(teams, models.PublicID(models.PrefixTeam, id))
	}

	return ConversationAttribute{
		ID: models.PublicID(models.PrefixAIAttribute, a.ID), Object: "conversation_attribute", Key: a.Key,
		Label: a.Label, Description: a.Description, DataType: a.DataType, Options: options, FilledBy: a.FilledBy,
		RequiredToClose: a.RequiredToClose, TeamIDs: teams, Status: a.Status, CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

/*
ConversationAttributeSource is who set a value and when.
*/
type ConversationAttributeSource struct {
	By   string    `json:"by"`
	ID   *string   `json:"id"`
	Name *string   `json:"name"`
	At   time.Time `json:"at"`
}

/*
ConversationAttributeValue is one attribute of a conversation, its
value, null while none is set, and who set it.
*/
type ConversationAttributeValue struct {
	Object    string                       `json:"object"`
	Attribute ConversationAttribute        `json:"attribute"`
	Value     json.RawMessage              `json:"value"`
	SetBy     *ConversationAttributeSource `json:"set_by"`
	Files     []Attachment                 `json:"files"`
}

/*
ConversationAttributeValues is a conversation's attribute values and
when the AI last read it.
*/
type ConversationAttributeValues struct {
	Object    string                       `json:"object"`
	Values    []ConversationAttributeValue `json:"values"`
	CheckedAt *time.Time                   `json:"checked_at"`
}

/*
NewConversationAttributeValues shapes a conversation's values.
*/
func NewConversationAttributeValues(f aiattributes.Found) ConversationAttributeValues {
	out := ConversationAttributeValues{
		Object: "conversation_attribute_values", Values: []ConversationAttributeValue{}, CheckedAt: f.CheckedAt,
	}
	for _, v := range f.Values {
		value, err := json.Marshal(v.Value)
		if err != nil {
			value = json.RawMessage("null")
		}

		item := ConversationAttributeValue{
			Object: "conversation_attribute_value", Attribute: NewConversationAttribute(v.Attribute), Value: value,
			Files: filesOfValue(f, v),
		}
		if s := v.Source; s != nil {
			item.SetBy = &ConversationAttributeSource{By: s.By, ID: orNull(s.ID), Name: orNull(s.Name), At: s.At}
		}

		out.Values = append(out.Values, item)
	}

	return out
}

/*
filesOfValue returns the files a file attribute's value holds, in its
order, none for other types.
*/
func filesOfValue(f aiattributes.Found, v aiattributes.Value) []Attachment {
	out := []Attachment{}

	list, _ := v.Value.([]any)
	for _, x := range list {
		id, _ := x.(string)
		if file, ok := f.Files[id]; ok {
			out = append(out, NewAttachment(file))
		}
	}

	return out
}
