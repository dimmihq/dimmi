/*
|--------------------------------------------------------------------------
| Conversation Context Resource
|--------------------------------------------------------------------------
|
| The customer panel's data in one shape: the contact with notes, the
| company with its attributes, identities, where the conversation began,
| the newest past conversations and how many look like duplicates. It is
| only ever served to members, so notes stay in.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationContext is what is known about a conversation's customer.
*/
type ConversationContext struct {
	Object            string            `json:"object"`
	Contact           Contact           `json:"contact"`
	Company           *ContextCompany   `json:"company"`
	Identities        []ContactIdentity `json:"identities"`
	Source            json.RawMessage   `json:"source"`
	PastConversations PastConversations `json:"past_conversations"`
	DuplicatesCount   int               `json:"duplicates_count"`
}

/*
ContextCompany is the customer's company with its attributes.
*/
type ContextCompany struct {
	CompanySummary

	CustomAttributes json.RawMessage `json:"custom_attributes"`
}

/*
PastConversations is the customer's newest other conversations and how
many they have, counted up to 100.
*/
type PastConversations struct {
	Data          []Conversation `json:"data"`
	TotalEstimate int            `json:"total_estimate"`
}

/*
NewConversationContext shapes the context.
*/
func NewConversationContext(c conversations.Context) ConversationContext {
	source := c.Source
	if len(source) == 0 {
		source = json.RawMessage(`{}`)
	}

	out := ConversationContext{
		Object: "conversation_context", Contact: NewContactDetail(c.Contact),
		Identities: NewContactIdentities(c.Identities), Source: source, DuplicatesCount: c.Duplicates,
		PastConversations: PastConversations{Data: NewConversations(c.Past), TotalEstimate: c.PastTotal},
	}

	if c.Contact.Company != nil {
		out.Company = new(newContextCompany(*c.Contact.Company))
	}

	return out
}

/*
newContextCompany shapes the company with its attributes.
*/
func newContextCompany(c models.Company) ContextCompany {
	attrs := c.CustomAttributes
	if len(attrs) == 0 {
		attrs = json.RawMessage(`{}`)
	}

	return ContextCompany{CompanySummary: NewCompanySummary(c), CustomAttributes: attrs}
}
