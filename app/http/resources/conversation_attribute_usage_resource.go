/*
|--------------------------------------------------------------------------
| Conversation Attribute Usage Resource
|--------------------------------------------------------------------------
|
| What uses a conversation attribute, which says whether it can be
| deleted.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/aiattributes"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NamedRecord is a workflow or a view by its id and name.
*/
type NamedRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
ConversationAttributeUsage is what uses an attribute: how many
conversations hold a value, and the workflows and views that read or
set it. An attribute is deleted only while nothing uses it.
*/
type ConversationAttributeUsage struct {
	Object        string        `json:"object"`
	Conversations int64         `json:"conversations"`
	Workflows     []NamedRecord `json:"workflows"`
	Views         []NamedRecord `json:"views"`
	Deletable     bool          `json:"deletable"`
}

/*
NewConversationAttributeUsage shapes what uses an attribute.
*/
func NewConversationAttributeUsage(u aiattributes.Usage) ConversationAttributeUsage {
	out := ConversationAttributeUsage{
		Object: "conversation_attribute_usage", Conversations: u.Conversations, Workflows: []NamedRecord{},
		Views: []NamedRecord{}, Deletable: u.Unused(),
	}
	for _, w := range u.Workflows {
		id := models.PublicID(models.PrefixWorkflow, w.ID)
		out.Workflows = append(out.Workflows, NamedRecord{ID: id, Name: w.Name})
	}

	for _, v := range u.Views {
		id := models.PublicID(models.PrefixConversationView, v.ID)
		out.Views = append(out.Views, NamedRecord{ID: id, Name: v.Name})
	}

	return out
}
