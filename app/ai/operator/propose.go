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
const proposeDescription = "Propose changes to conversations, customers and procedures as one changeset a " +
	"teammate reviews: tag or untag, assign to a teammate or team, set priority, open or close, rename, add " +
	"an internal note, update a customer's name, edit a procedure's working copy, or add a regression test " +
	"to a procedure from a conversation that went wrong. Help articles have their own tools: " +
	"propose_article_edits and propose_new_article. Give a short title and the reason, both in plain words a " +
	"teammate reads at a glance."

/*
reasonHelp tells the model how to write the reason of a suggestion,
which a teammate reads on its card and above the change under review.
*/
const reasonHelp = "Why, written so a 12-year-old could follow it. Two or three very short sentences in " +
	"everyday words, one idea in each: what is wrong today, what is true, what this changes. Like: 'This " +
	"page says refunds take 30 days. **Refund timing** says 14 days. This fix makes them match.' Name " +
	"another article by its title in **bold**, and conversations by number. When the change picks " +
	"something (a teammate, a team, a tag, a priority), say why that one, from what you saw: 'Maya " +
	"answered the last three billing questions.' Never join ideas with 'so', 'which' or 'while'. No " +
	"jargon, no ids, no tool names."

/*
proposeSchema is the schema of a proposal.
*/
const proposeSchema = `{"type":"object","required":["suggestion_title","reason","operations"],"properties":{
"suggestion_title":{"type":"string","maxLength":200,"description":"What it does in a few everyday words, like ` +
	`'Route to Billing'."},
"reason":{"type":"string","maxLength":2000,"description":"` + reasonHelp + `"},
"dedup_key":{"type":"string","maxLength":200,"description":"A stable key for what this suggests, like ` +
	`'qa:prc_...:step-2'. A suggestion with the same key that still waits, or was declined in the last 30 ` +
	`days, is not made again."},
"operations":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"object","required":["kind"],
"properties":{
"kind":{"type":"string","enum":["conversation.tag","conversation.untag","conversation.assign",
"conversation.priority","conversation.status","conversation.title","conversation.note","contact.update",
"procedure.edit_draft","procedure.create_test_case"]},
"conversation_number":{"type":"integer","description":"Conversation number, such as 123 for #123; leave ` +
	`out for this thread's."},
"tags":{"type":"array","items":{"type":"string"},"description":"For conversation.tag and untag: tag names."},
"member_id":{"type":"string","description":"For conversation.assign: teammate id (mem_...), or none."},
"team_id":{"type":"string","description":"For conversation.assign: team id (team_...), or none."},
"priority":{"type":"string","enum":["none","low","medium","high","urgent"],
"description":"For conversation.priority."},
"status":{"type":"string","enum":["open","closed"],"description":"For conversation.status."},
"new_subject":{"type":"string","description":"For conversation.title: the conversation's new subject."},
"note_text":{"type":"string","description":"For conversation.note: the note's text."},
"contact_id":{"type":"string","description":"For contact.update: customer id (ctc_...); leave out for this ` +
	`conversation's."},
"customer_name":{"type":"string","description":"For contact.update: the customer's new name."},
"procedure_id":{"type":"string","description":"For the procedure kinds: procedure id (prc_...)."},
"procedure_definition":{"type":"object","description":"For procedure.edit_draft: the procedure's whole new working ` +
	`copy, as get_procedure_by_id shows it."},
"test_case":{"type":"object","description":"For procedure.create_test_case: the test, with name, messages ` +
	`([{text}], the customer's turns), persona ({verified, external_id, channel, attributes}), action_mocks ` +
	`({action name: {status: HTTP status as a number such as 200, body: any JSON}}), assertions ([{type, ...}]: ` +
	`procedure_matched; action_called and action_not_called with action (its name); action_order with action ` +
	`and before; branch_taken with step_id and branch; handed_off with handed_off true or false; ` +
	`reply_contains and reply_not_contains with text) and source_conversation_number."}
}}}}}`

/*
proposal is a proposal as the model sent it.
*/
type proposal struct {
	Title      string    `json:"suggestion_title"`
	Reason     string    `json:"reason"`
	DedupKey   string    `json:"dedup_key"`
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
	Collection   string          `json:"collection"`
	Visibility   string          `json:"visibility"`
	Article      string          `json:"article"`
	Procedure    string          `json:"procedure"`
	Definition   json.RawMessage `json:"definition"`
	TestCase     json.RawMessage `json:"test_case"`
	Edits        passageEdits    `json:"edits"`
	MemberID     string          `json:"member_id"`
	TeamID       string          `json:"team_id"`
	ContactID    string          `json:"contact_id"`
	HelpCenterID string          `json:"help_center_id"`
	CollectionID string          `json:"collection_id"`
	ArticleID    string          `json:"article_id"`
	ProcedureID  string          `json:"procedure_id"`
	Number       int64           `json:"conversation_number"`
	Subject      string          `json:"new_subject"`
	NoteText     string          `json:"note_text"`
	CustomerName string          `json:"customer_name"`
	NewWorking   json.RawMessage `json:"procedure_definition"`
}

/*
named is the operation with every id under one name, whether the model
sent it as article_id or, as older prompts did, as article.
*/
func (in opInput) named() opInput {
	in.Member, in.Team = either(in.MemberID, in.Member), either(in.TeamID, in.Team)
	in.Contact, in.Procedure = either(in.ContactID, in.Contact), either(in.ProcedureID, in.Procedure)
	in.HelpCenter, in.Collection = either(in.HelpCenterID, in.HelpCenter), either(in.CollectionID, in.Collection)
	in.Article = either(in.ArticleID, in.Article)
	in.Name = either(in.CustomerName, in.Name)
	if in.Number != 0 {
		in.Conversation = in.Number
	}
	if len(in.NewWorking) > 0 {
		in.Definition = in.NewWorking
	}
	if in.Kind == KindTitle {
		in.Title = either(in.Subject, in.Title)
	}
	if in.Kind == KindNote {
		in.Body = either(in.NoteText, in.Body)
	}
	return in
}

/*
check refuses a proposal without a title or operations.
*/
func (p proposal) check() error {
	title := strings.TrimSpace(p.Title)
	if title == "" || len([]rune(title)) > 200 {
		return fmt.Errorf("%w: \"suggestion_title\" is missing; give 1 to 200 characters", errBadArguments)
	}
	if len(p.Operations) == 0 || len(p.Operations) > maxOperations {
		return fmt.Errorf("%w: \"operations\" must hold 1 to %d operations", errBadArguments, maxOperations)
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
			raw = raw.named()
			if !slices.Contains(Kinds(), raw.Kind) || raw.Kind == KindAction {
				return fmt.Errorf("%w: operation %d has an unknown kind %q", errBadArguments, i+1, raw.Kind)
			}
			if !offered(s.settings, raw.Kind) {
				return fmt.Errorf("%w: %s changes are switched off in this workspace", errBadArguments, raw.Kind)
			}
			op, err := r.resolve(ctx, raw)
			if err != nil {
				return placed(err, in, i)
			}
			out = append(out, op)
		}
		return nil
	})
	return out, err
}

/*
placed says which operation an error is about when there are several.
*/
func placed(err error, in []opInput, i int) error {
	if len(in) == 1 {
		return err
	}
	return fmt.Errorf("operation %d (%s): %w", i+1, in[i].Kind, err)
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
