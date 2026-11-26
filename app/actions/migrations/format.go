/*
|--------------------------------------------------------------------------
| Import Format, Version 1
|--------------------------------------------------------------------------
|
| dimmi's own shape for history brought in from another tool. A client
| translates whatever the tool calls things into these records, so the
| server never learns which tool it was. Every record carries the id it
| had in the tool, its source_id, and names the records it points at by
| their source ids too. Times are RFC 3339. Bodies are Markdown, except
| a saved reply's, which is dimmi's Liquid. The same shape is published
| as a JSON Schema and documented for clients.
|
*/

package migrations

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
FormatVersion is the version of the import format this server reads.
*/
const FormatVersion = 1

/*
The kinds of record, in the order a client sends them so that most
references already exist when they arrive.
*/
const (
	KindTeammate            = "teammate"
	KindTeam                = "team"
	KindTag                 = "tag"
	KindAttributeDefinition = "attribute_definition"
	KindCompany             = "company"
	KindContact             = "contact"
	KindContactNote         = "contact_note"
	KindTicketType          = "ticket_type"
	KindHelpCenter          = "help_center"
	KindCollection          = "collection"
	KindArticle             = "article"
	KindSnippet             = "snippet"
	KindSavedReply          = "saved_reply"
	KindSchedule            = "schedule"
	KindWorkflow            = "workflow"
	KindConversation        = "conversation"
	KindTicket              = "ticket"
)

/*
kindFile is the ledger kind of an uploaded file, and kindTicketState
of a ticket type's state, which records name as "<type>/<state>".
*/
const (
	kindFile        = "file"
	kindTicketState = "ticket_state"
)

/*
Kinds lists every kind of record, in sending order.
*/
func Kinds() []string {
	return []string{
		KindTeammate, KindTeam, KindTag, KindAttributeDefinition, KindCompany, KindContact, KindContactNote,
		KindTicketType, KindHelpCenter, KindCollection, KindArticle, KindSnippet, KindSavedReply, KindSchedule,
		KindWorkflow, KindConversation, KindTicket,
	}
}

/*
Base is what every record carries: its id in the other tool and when it
last changed there.
*/
type Base struct {
	SourceID        string     `json:"source_id"`
	SourceUpdatedAt *time.Time `json:"source_updated_at,omitempty"`
}

/*
Author is who wrote an entry or made a change: a teammate or a contact
by source id, an AI agent with the label it showed, or the system.
*/
type Author struct {
	Type  string `json:"type"`
	Ref   string `json:"ref,omitempty"`
	Label string `json:"label,omitempty"`
}

/*
The kinds of author.
*/
const (
	AuthorTeammate = "teammate"
	AuthorContact  = "contact"
	AuthorAIAgent  = "ai_agent"
	AuthorSystem   = "system"
)

/*
PrefixOf returns the public id prefix of what a record of the kind
becomes, an attachment's for a file.
*/
func PrefixOf(kind string) models.Prefix {
	if p, ok := prefixes()[kind]; ok {
		return p
	}

	return models.PrefixAttachment
}

/*
prefixes maps each kind to the public id prefix of what it becomes.
*/
func prefixes() map[string]models.Prefix {
	return map[string]models.Prefix{
		KindTeammate: models.PrefixMember, KindTeam: models.PrefixTeam, KindTag: models.PrefixTag,
		KindAttributeDefinition: models.PrefixAttribute, KindCompany: models.PrefixCompany,
		KindContact: models.PrefixContact, KindContactNote: models.PrefixContactNote,
		KindTicketType: models.PrefixTicketType, KindTicket: models.PrefixTicket,
		KindHelpCenter: models.PrefixHelpCenter, KindCollection: models.PrefixCollection,
		KindArticle: models.PrefixArticle, KindSnippet: models.PrefixArticle, KindSavedReply: models.PrefixSavedReply,
		KindSchedule: models.PrefixSchedule, KindConversation: models.PrefixConversation,
	}
}
