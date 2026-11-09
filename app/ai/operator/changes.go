/*
|--------------------------------------------------------------------------
| Changes
|--------------------------------------------------------------------------
|
| Everything Operator changes is an operation inside a changeset. An
| operation names what it touches by row id and holds what it sets; the
| values it replaced are read when it is applied, not when it was
| proposed, so a revert puts back what was really there. Each operation
| keeps a short label a teammate reads on the card.
|
*/

package operator

import (
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Kind names what one operation does. It is a plain string so a switch
may handle only the kinds it cares about.
*/
type Kind = string

/*
The operations Operator can propose.
*/
const (
	KindTag          Kind = "conversation.tag"
	KindUntag        Kind = "conversation.untag"
	KindAssign       Kind = "conversation.assign"
	KindPriority     Kind = "conversation.priority"
	KindStatus       Kind = "conversation.status"
	KindTitle        Kind = "conversation.title"
	KindNote         Kind = "conversation.note"
	KindContact      Kind = "contact.update"
	KindArticleNew   Kind = "article.create_draft"
	KindArticleEdit  Kind = "article.edit_draft"
	KindProcedureSet Kind = "procedure.edit_draft"
	KindAction       Kind = "action.run"
)

/*
Kinds lists every operation kind.
*/
func Kinds() []Kind {
	return []Kind{
		KindTag, KindUntag, KindAssign, KindPriority, KindStatus, KindTitle, KindNote, KindContact,
		KindArticleNew, KindArticleEdit, KindProcedureSet, KindAction,
	}
}

/*
GroupOf returns the policy group an operation kind belongs to.
*/
func GroupOf(k Kind) Group {
	switch {
	case k == KindNote:
		return GroupNotes
	case strings.HasPrefix(k, "conversation."):
		return GroupConversations
	case k == KindContact:
		return GroupContacts
	case strings.HasPrefix(k, "article."):
		return GroupArticles
	case k == KindAction:
		return GroupActions
	default:
		return GroupProcedures
	}
}

/*
Operation is one change. Only the fields its kind reads are set:
Conversation for conversation kinds, Contact, Article and HelpCenter,
Procedure; Tags for tagging; Member and Team for assignment, Unassign
to clear both; Value for priority, status, title, note body or contact
name; Title and Body for articles; Definition for procedures, or an
action's arguments; Action and Version for the workspace action to run.
Before
and After are filled when it is applied, Created names a note or
article it made, and Outcome says what became of it.
*/
type Operation struct {
	Kind         Kind            `json:"kind"`
	Label        string          `json:"label"`
	Conversation *ids.UUID       `json:"conversation_id,omitempty"`
	Number       int64           `json:"number,omitempty"`
	Contact      *ids.UUID       `json:"contact_id,omitempty"`
	Article      *ids.UUID       `json:"article_id,omitempty"`
	HelpCenter   *ids.UUID       `json:"help_center_id,omitempty"`
	Collection   *ids.UUID       `json:"collection_id,omitempty"`
	Procedure    *ids.UUID       `json:"procedure_id,omitempty"`
	Action       *ids.UUID       `json:"action_id,omitempty"`
	Version      *ids.UUID       `json:"action_version_id,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	Member       *ids.UUID       `json:"member_id,omitempty"`
	Team         *ids.UUID       `json:"team_id,omitempty"`
	Unassign     bool            `json:"unassign,omitempty"`
	Value        *string         `json:"value,omitempty"`
	Title        *string         `json:"title,omitempty"`
	Body         *string         `json:"body,omitempty"`
	Definition   json.RawMessage `json:"definition,omitempty"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
	Created      *ids.UUID       `json:"created_id,omitempty"`
	Outcome      string          `json:"outcome,omitempty"`
}

/*
The outcomes an operation records.
*/
const (
	OutcomeApplied  = "applied"
	OutcomeReverted = "reverted"
	OutcomeSkipped  = "skipped"
)

/*
Operations reads a changeset's stored operations.
*/
func Operations(raw json.RawMessage) []Operation {
	var ops []Operation
	_ = json.Unmarshal(raw, &ops)
	return ops
}

/*
encodeOperations stores operations as JSON.
*/
func encodeOperations(ops []Operation) json.RawMessage {
	raw, _ := json.Marshal(ops)
	return raw
}

/*
groupsOf returns the groups a changeset's operations touch.
*/
func groupsOf(ops []Operation) []Group {
	seen := map[Group]bool{}
	var out []Group
	for _, op := range ops {
		g := GroupOf(op.Kind)
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

/*
autoApplies reports whether every group a changeset touches applies at
once under the settings.
*/
func autoApplies(s Settings, ops []Operation) bool {
	for _, g := range groupsOf(ops) {
		if s.PolicyFor(g) != PolicyAuto {
			return false
		}
	}
	return len(ops) > 0
}

/*
offered reports whether a kind's group is not switched off.
*/
func offered(s Settings, k Kind) bool {
	return s.PolicyFor(GroupOf(k)) != PolicyOff
}

/*
rawJSON encodes a value for an operation's before or after.
*/
func rawJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
