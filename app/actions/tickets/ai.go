/*
|--------------------------------------------------------------------------
| AI Help
|--------------------------------------------------------------------------
|
| The AI reads a conversation to fill a ticket's fields, each by the
| hint its field gives, and its values wait as suggestions until a
| person accepts them, unless the field fills itself. It matches a new
| conversation against open trackers by meaning, so the panel suggests
| where the report belongs. Without a model every answer is empty.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Filler is the model behind ticket AI.
*/
type Filler interface {
	/*
		Fill proposes field values from the conversation.
	*/
	Fill(ctx context.Context, q FillQuestion) (map[string]any, error)

	/*
		Match returns the candidates the conversation is about, best first.
	*/
	Match(ctx context.Context, q MatchQuestion) ([]ids.UUID, error)
}

/*
FillField is one field the model may fill: its key, label, data type,
options and the team's hint.
*/
type FillField struct {
	Key      string
	Label    string
	DataType string
	Options  []string
	Hint     string
}

/*
FillQuestion is a conversation's words and the fields to fill.
*/
type FillQuestion struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	Text           string
	Fields         []FillField
}

/*
maxAIText is the most of a conversation the model reads.
*/
const maxAIText = 6000

/*
Suggest proposes field values for a type from a conversation the member
may see, without storing them.
*/
func (t *Tickets) Suggest(
	ctx context.Context, m authz.Membership, ref conversations.Ref, typeID ids.UUID,
) (map[string]any, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var (
		question FillQuestion
		fields   []models.TicketField
	)

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		setup, err := loadSetup(ctx, q, m.Workspace.ID, t.Clock.Now())
		if err != nil {
			return err
		}

		fields = setup.FieldsOf(typeID)
		question, err = fillQuestion(ctx, q, c, fields)

		return err
	})
	if err != nil {
		return nil, err
	}

	return t.fill(ctx, question, fields)
}

/*
fill asks the model and keeps only values that fit their fields.
*/
func (t *Tickets) fill(
	ctx context.Context, question FillQuestion, fields []models.TicketField,
) (map[string]any, error) {
	if t.Filler == nil || len(question.Fields) == 0 || strings.TrimSpace(question.Text) == "" {
		return map[string]any{}, nil
	}

	raw, err := t.Filler.Fill(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("fill ticket fields: %w", err)
	}

	out := map[string]any{}
	for k, v := range raw {
		f, ok := fieldByKey(fields, k)
		if !ok || f.ArchivedAt != nil {
			continue
		}

		if fitted, problem := fit(f, v); problem == "" {
			out[k] = fitted
		}
	}

	return out, nil
}

/*
fillQuestion gathers the conversation's words and the fields with AI
hints.
*/
func fillQuestion(
	ctx context.Context, q *models.Queries, c models.Conversation, fields []models.TicketField,
) (FillQuestion, error) {
	text, err := transcript(ctx, q, c)
	if err != nil {
		return FillQuestion{}, err
	}

	out := FillQuestion{WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Text: text}

	for _, f := range fields {
		if f.ArchivedAt != nil || f.DataType == TypeFile {
			continue
		}

		hint := ""
		if f.AiHint != nil {
			hint = *f.AiHint
		}

		out.Fields = append(out.Fields, FillField{
			Key: f.Key, Label: f.Label, DataType: f.DataType, Options: f.Options, Hint: hint,
		})
	}

	return out, nil
}

/*
transcript writes the conversation's messages, customer and team, cut
to what the model reads.
*/
func transcript(ctx context.Context, q *models.Queries, c models.Conversation) (string, error) {
	msgs, err := q.ListRecentConversationText(ctx, models.ListRecentConversationTextParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil {
		return "", fmt.Errorf("read conversation text: %w", err)
	}

	var b strings.Builder
	if c.Subject != nil {
		fmt.Fprintf(&b, "Subject: %s\n", *c.Subject)
	}

	for _, msg := range slices.Backward(msgs) {
		fmt.Fprintf(&b, "%s: %s\n", msg.AuthorType, msg.Body)
	}

	s := b.String()
	if r := []rune(s); len(r) > maxAIText {
		s = string(r[len(r)-maxAIText:])
	}

	return s, nil
}

/*
autoFill stores AI values of a new ticket: as values for fields that
fill themselves, or for every field when the type allows it, and as
suggestions for the rest.
*/
func autoFill(ty models.TicketType, fields []models.TicketField, values, proposed map[string]any) map[string]any {
	suggestions := map[string]any{}

	for k, v := range proposed {
		if cur, ok := values[k]; ok && !empty(cur) {
			continue
		}

		f, _ := fieldByKey(fields, k)
		if ty.AiAutoFill || f.AiAuto {
			values[k] = v
		} else {
			suggestions[k] = v
		}
	}

	return suggestions
}
