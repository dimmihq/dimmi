/*
|--------------------------------------------------------------------------
| Proposals
|--------------------------------------------------------------------------
|
| The model names things the way it read them: conversations by number,
| everything else by public id. A proposal is checked before it is
| stored: every conversation is one the teammate can see, every id is
| real, every value is one the product accepts, and each operation gets
| the label a teammate reads on the card.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
proposeDescription tells the model how to propose changes.
*/
const proposeDescription = "Propose changes to the workspace as one changeset a teammate reviews: tag or untag, " +
	"assign to a teammate or team, set priority, open or close, rename, add an internal note, update a " +
	"customer's name, create a help article draft, edit an article's draft, or edit a procedure's working " +
	"copy. Give a short title and the reason."

/*
proposeSchema is the schema of a proposal.
*/
const proposeSchema = `{"type":"object","required":["title","reason","operations"],"properties":{
"title":{"type":"string","maxLength":200,"description":"What the changeset does, like 'Route to Billing'."},
"reason":{"type":"string","maxLength":2000},
"operations":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"object","required":["kind"],
"properties":{
"kind":{"type":"string","enum":["conversation.tag","conversation.untag","conversation.assign",
"conversation.priority","conversation.status","conversation.title","conversation.note","contact.update",
"article.create_draft","article.edit_draft","procedure.edit_draft"]},
"conversation":{"type":"integer","description":"Conversation number; leave out for this thread's."},
"tags":{"type":"array","items":{"type":"string"},"description":"Tag names, for tag and untag."},
"member":{"type":"string","description":"Teammate id (mem_...) to assign, or none."},
"team":{"type":"string","description":"Team id (team_...) to assign, or none."},
"priority":{"type":"string","enum":["none","low","medium","high","urgent"]},
"status":{"type":"string","enum":["open","closed"]},
"title":{"type":"string","description":"Conversation subject or article title."},
"body":{"type":"string","description":"Note text or article Markdown."},
"contact":{"type":"string","description":"Customer id (ctc_...); leave out for this conversation's."},
"name":{"type":"string","description":"The customer's new name."},
"help_center":{"type":"string","description":"Help center id (hc_...); leave out for the default."},
"article":{"type":"string","description":"Article id (art_...)."},
"procedure":{"type":"string","description":"Procedure id (prc_...)."},
"definition":{"type":"object","description":"The procedure's whole new working copy, as get_procedure shows it."}
}}}}}`

/*
proposal is a proposal as the model sent it.
*/
type proposal struct {
	Title      string    `json:"title"`
	Reason     string    `json:"reason"`
	Operations []opInput `json:"operations"`
}

/*
opInput is one operation as the model sent it.
*/
type opInput struct {
	Kind         Kind            `json:"kind"`
	Conversation int64           `json:"conversation"`
	Tags         []string        `json:"tags"`
	Member       string          `json:"member"`
	Team         string          `json:"team"`
	Priority     string          `json:"priority"`
	Status       string          `json:"status"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Contact      string          `json:"contact"`
	Name         string          `json:"name"`
	HelpCenter   string          `json:"help_center"`
	Article      string          `json:"article"`
	Procedure    string          `json:"procedure"`
	Definition   json.RawMessage `json:"definition"`
}

/*
check refuses a proposal without a title or operations.
*/
func (p proposal) check() error {
	title := strings.TrimSpace(p.Title)
	if title == "" || len([]rune(title)) > 200 {
		return fmt.Errorf("%w: a title of 1 to 200 characters is required", errBadArguments)
	}
	if len(p.Operations) == 0 || len(p.Operations) > maxOperations {
		return fmt.Errorf("%w: give 1 to %d operations", errBadArguments, maxOperations)
	}
	return nil
}

/*
operations checks every operation of a proposal and resolves what it
names.
*/
func (e *Engine) operations(ctx context.Context, s *session, in []opInput) ([]Operation, error) {
	out := make([]Operation, 0, len(in))
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		r := resolver{e: e, s: s, q: models.New(tx)}
		for i, raw := range in {
			if !slices.Contains(Kinds(), raw.Kind) || raw.Kind == KindAction {
				return fmt.Errorf("%w: operation %d has an unknown kind %q", errBadArguments, i+1, raw.Kind)
			}
			if !offered(s.settings, raw.Kind) {
				return fmt.Errorf("%w: %s changes are switched off in this workspace", errBadArguments, raw.Kind)
			}
			op, err := r.resolve(ctx, raw)
			if err != nil {
				return fmt.Errorf("operation %d (%s): %w", i+1, raw.Kind, err)
			}
			out = append(out, op)
		}
		return nil
	})
	return out, err
}

/*
resolver checks operations inside one transaction.
*/
type resolver struct {
	e *Engine
	s *session
	q *models.Queries
}

/*
resolve checks one operation by its kind.
*/
func (r resolver) resolve(ctx context.Context, in opInput) (Operation, error) {
	switch GroupOf(in.Kind) {
	case GroupConversations, GroupNotes:
		return r.conversationOp(ctx, in)
	case GroupContacts:
		return r.contactOp(ctx, in)
	case GroupArticles:
		return r.articleOp(ctx, in)
	default:
		return r.procedureOp(ctx, in)
	}
}

/*
conversation finds the conversation an operation names, among those the
teammate can see.
*/
func (r resolver) conversation(ctx context.Context, number int64) (models.Conversation, error) {
	ref, err := ref(r.s, number)
	if err != nil {
		return models.Conversation{}, err
	}
	return conversations.Find(ctx, r.q, r.s.m, ref)
}

/*
conversationOp checks an operation on a conversation.
*/
func (r resolver) conversationOp(ctx context.Context, in opInput) (Operation, error) {
	c, err := r.conversation(ctx, in.Conversation)
	if err != nil {
		return Operation{}, err
	}
	op := Operation{Kind: in.Kind, Conversation: &c.ID, Number: c.Number}
	at := fmt.Sprintf("#%d", c.Number)
	switch in.Kind {
	case KindTag, KindUntag:
		return tagOp(op, in, at)
	case KindAssign:
		return r.assignOp(ctx, op, in, at)
	case KindPriority:
		return valueOp(op, in.Priority, enumValues(enums.ConversationPriorities()), "Set "+at+" priority to %s")
	case KindStatus:
		return valueOp(op, in.Status, []string{"open", "closed"}, map[string]string{
			"open": "Reopen " + at, "closed": "Close " + at,
		}[in.Status]+"%.0s")
	case KindTitle:
		return textOp(op, in.Title, 200, "Rename "+at+" to “%s”")
	default:
		return textOp(op, in.Body, 10000, "Add an internal note to "+at+": %s")
	}
}

/*
tagOp checks a tag or untag operation.
*/
func tagOp(op Operation, in opInput, at string) (Operation, error) {
	for _, t := range in.Tags {
		if name := strings.TrimSpace(t); name != "" && len([]rune(name)) <= 50 && !slices.Contains(op.Tags, name) {
			op.Tags = append(op.Tags, name)
		}
	}
	if len(op.Tags) == 0 || len(op.Tags) > 10 {
		return op, fmt.Errorf("%w: give 1 to 10 tag names", errBadArguments)
	}
	verb := map[Kind]string{KindTag: "Tag %s with %s", KindUntag: "Remove %[2]s from %[1]s"}[op.Kind]
	op.Label = fmt.Sprintf(verb, at, strings.Join(op.Tags, ", "))
	return op, nil
}

/*
assignOp checks an assignment, resolving the teammate and team.
*/
func (r resolver) assignOp(ctx context.Context, op Operation, in opInput, at string) (Operation, error) {
	var names []string
	if strings.EqualFold(in.Member, "none") && strings.EqualFold(in.Team, "none") {
		op.Unassign, op.Label = true, "Unassign "+at
		return op, nil
	}
	if in.Member != "" && !strings.EqualFold(in.Member, "none") {
		id, err := r.e.publicID(in.Member, models.PrefixMember)
		if err != nil {
			return op, err
		}
		who, err := r.q.FindAuditActor(ctx, models.FindAuditActorParams{WorkspaceID: r.s.m.Workspace.ID, ID: id})
		if err != nil {
			return op, fmt.Errorf("find teammate: %w", err)
		}
		op.Member, names = &id, append(names, who.Name)
	}
	if in.Team != "" && !strings.EqualFold(in.Team, "none") {
		id, err := r.e.publicID(in.Team, models.PrefixTeam)
		if err != nil {
			return op, err
		}
		team, err := r.q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: r.s.m.Workspace.ID, ID: id})
		if err != nil {
			return op, fmt.Errorf("find team: %w", err)
		}
		op.Team, names = &id, append(names, team.Team.Name)
	}
	if len(names) == 0 {
		return op, fmt.Errorf("%w: name a teammate or a team", errBadArguments)
	}
	op.Label = fmt.Sprintf("Assign %s to %s", at, strings.Join(names, " and "))
	return op, nil
}

/*
valueOp checks an operation that sets one of a few values.
*/
func valueOp(op Operation, value string, allowed []string, label string) (Operation, error) {
	if !slices.Contains(allowed, value) {
		return op, fmt.Errorf("%w: use one of %s", errBadArguments, strings.Join(allowed, ", "))
	}
	op.Value, op.Label = &value, fmt.Sprintf(label, value)
	return op, nil
}

/*
textOp checks an operation that sets a text of at most n characters.
*/
func textOp(op Operation, text string, n int, label string) (Operation, error) {
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > n {
		return op, fmt.Errorf("%w: the text must be 1 to %d characters", errBadArguments, n)
	}
	op.Value, op.Label = &text, fmt.Sprintf(label, truncate(text, 80))
	return op, nil
}

/*
enumValues lists an enum's values as strings.
*/
func enumValues[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

/*
sharedConversation is the conversation every operation is about, nil
when they differ or some are about something else.
*/
func sharedConversation(ops []Operation) *ids.UUID {
	var shared *ids.UUID
	for _, op := range ops {
		if op.Conversation == nil || (shared != nil && *shared != *op.Conversation) {
			return nil
		}
		shared = op.Conversation
	}
	return shared
}
