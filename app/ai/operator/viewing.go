/*
|--------------------------------------------------------------------------
| What The Teammate Has Open
|--------------------------------------------------------------------------
|
| A teammate talks to Operator from a page: an article, a procedure,
| often with a suggestion waiting on it. Each question says what is
| open, so Operator can explain a suggestion, and so a change the
| teammate asks for lands where they are looking. A change to an article
| or procedure whose suggestion is open is added to that suggestion in
| place: the page shows one set of changes, the teammate keeps asking
| until it reads right, then accepts it. With no suggestion open, the
| change becomes a new one on the same page.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Viewing is what the teammate has open when they ask: an article or a
procedure, and the suggestion shown on it.
*/
type Viewing struct {
	Article   *ids.UUID
	Procedure *ids.UUID
	Changeset *ids.UUID
}

/*
lookAt reads what the teammate has open and returns what the model
reads with their question, empty when nothing is open: every detail of
the page, so Operator never has to look it up or copy an id from
memory. That is the article or procedure with its id and text, and the
suggestion shown on it with its id, where it came from, why, what it
changes and the text as suggested. The suggestion is kept on the
session only while it waits and the teammate may see it.
*/
func (e *Engine) lookAt(ctx context.Context, s *session) string {
	v := s.viewing
	var b strings.Builder
	if c, ok := e.viewedSuggestion(ctx, s); ok {
		s.viewed = &c
	}
	if v.Article != nil {
		if a, err := e.Services.Articles.Show(ctx, s.m, *v.Article); err == nil {
			id := models.PublicID(models.PrefixArticle, a.ID)
			fmt.Fprintf(&b, "The help article %q is open.\narticle_id: %s\nStatus: %s%s\n", a.Title, id, a.Status,
				pendingMark(a))
			if _, suggested := s.suggested(a.ID); !suggested {
				fmt.Fprintf(&b, "The article as it reads now:\n%s\n", truncate(a.BodyMarkdown, 20000))
			}
			fmt.Fprintf(&b, "To change it, call propose_article_edits with article_id %s.\n\n", id)
		}
	}
	if v.Procedure != nil {
		if p, err := e.Services.Procedures.Show(ctx, s.m, *v.Procedure); err == nil {
			id := models.PublicID(models.PrefixProcedure, p.ID)
			fmt.Fprintf(&b, "The procedure %q is open.\nprocedure_id: %s\nStatus: %s\nWorking copy: %s\n"+
				"To change it, call propose_changes with a procedure.edit_draft for procedure_id %s.\n\n", p.Name, id,
				p.Status, truncate(string(p.Draft), 20000), id)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	if s.viewed != nil {
		b.WriteString("A suggestion waits for review on it, shown on the teammate's page:\n")
		b.WriteString(e.suggestionText(ctx, s, *s.viewed, e.madeBy(ctx, s, *s.viewed)))
		b.WriteString("\nChanges you make to this article or procedure are added to that suggestion, so quote " +
			"passages from the text shown above as suggested, not from the published article.\n")
	}
	return "[What the teammate has open right now. A change they ask for is about it unless they say otherwise. " +
		"Everything you need is here: use these ids exactly as written, and do not look this up again.\n\n" +
		strings.TrimSpace(b.String()) + "\nWhat you change shows on their page at once, for them to accept.]"
}

/*
madeBy says where a suggestion came from: a routine, by name, or a chat
with Operator.
*/
func (e *Engine) madeBy(ctx context.Context, s *session, c models.OperatorChangeset) string {
	if c.RoutineID == nil {
		return "in a chat with Operator"
	}
	made := "by a routine"
	_ = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		routine, err := models.New(tx).FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{
			WorkspaceID: s.m.Workspace.ID, ID: *c.RoutineID,
		})
		if err == nil {
			made = fmt.Sprintf("by the routine %q", routine.Name)
		}
		return nil
	})
	return made
}

/*
suggestionText is everything about one suggestion: its id, status,
where it came from, why, and what each of its changes does.
*/
func (e *Engine) suggestionText(ctx context.Context, s *session, c models.OperatorChangeset, made string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Suggestion %q\nsuggestion_id: %s\nStatus: %s\nMade %s on %s\nReason: %s\n", c.Title,
		models.PublicID(models.PrefixOperatorChangeset, c.ID), c.Status, made,
		c.CreatedAt.UTC().Format("2006-01-02"), c.Reason)
	for _, op := range Operations(c.Operations) {
		fmt.Fprintf(&b, "\n%s\n", op.Label)
		e.describeChange(ctx, s, &b, op)
	}
	return b.String()
}

/*
viewedSuggestion is the suggestion the teammate has open, when it still
waits, is no dry run's preview, and they may see it.
*/
func (e *Engine) viewedSuggestion(ctx context.Context, s *session) (models.OperatorChangeset, bool) {
	if s.viewing.Changeset == nil {
		return models.OperatorChangeset{}, false
	}
	var c models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		c, err = q.FindOperatorChangeset(ctx, models.FindOperatorChangesetParams{
			WorkspaceID: s.m.Workspace.ID, ID: *s.viewing.Changeset,
		})
		if err != nil {
			return err
		}
		return Visible(ctx, q, s.m, c)
	})
	return c, err == nil && c.Status == StatusPending && !c.Preview
}

/*
suggested is the text of an article as the open suggestion would leave
it, which further edits build on.
*/
func (s *session) suggested(article ids.UUID) (string, bool) {
	if s.viewed == nil {
		return "", false
	}
	for _, op := range Operations(s.viewed.Operations) {
		if op.Kind == KindArticleEdit && op.Article != nil && *op.Article == article && op.Body != nil {
			return *op.Body, true
		}
	}
	return "", false
}

/*
amend adds a change to the open suggestion when it changes the same
article or procedure, in place. It reports whether it did; a change to
anything else becomes a suggestion of its own.
*/
func (e *Engine) amend(ctx context.Context, s *session, in proposal, ops []Operation) (result, bool, error) {
	if s.viewed == nil || len(ops) != 1 || targetOf(ops[0]) == "" {
		return result{}, false, nil
	}
	stored := Operations(s.viewed.Operations)
	at := -1
	for i, op := range stored {
		if op.Kind == ops[0].Kind && targetOf(op) == targetOf(ops[0]) {
			at = i
		}
	}
	if at < 0 {
		return result{}, false, nil
	}
	change := ops[0]
	if change.Body != nil {
		stored[at].Body = change.Body
	}
	if change.Title != nil {
		stored[at].Title = change.Title
	}
	if len(change.Definition) > 0 {
		stored[at].Definition = change.Definition
	}
	reason := s.viewed.Reason
	if more := strings.TrimSpace(in.Reason); more != "" && !strings.Contains(reason, more) {
		reason = truncate(strings.TrimSpace(reason+" "+more), 2000)
	}
	var row models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).ReviseOperatorChangeset(ctx, models.ReviseOperatorChangesetParams{
			WorkspaceID: s.m.Workspace.ID, ID: s.viewed.ID, Operations: encodeOperations(stored), Reason: reason,
			Now: e.Clock.Now(),
		})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return result{}, false, nil
	}
	if err != nil {
		return result{}, false, fmt.Errorf("revise changeset: %w", err)
	}
	s.viewed = &row
	public := models.PublicID(models.PrefixOperatorChangeset, row.ID)
	return result{
		text: fmt.Sprintf("Added to the suggestion %q (%s) the teammate has open; their page now shows it with "+
			"your change, waiting for them to accept.%s", row.Title, public, notesOf(ops)),
		step: Step{Tool: "propose_changes", Summary: "Updated: " + row.Title, Changeset: public},
	}, true, nil
}

/*
suggestionTool is the tool that reads one suggestion.
*/
func (e *Engine) suggestionTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: "get_suggestion_by_id",
		Description: "Read one suggested change: what it would change, why, what made it, and for an article " +
			"the lines it removes and adds with the article's full text as suggested. With no arguments it " +
			"reads the suggestion the teammate has open.",
		Schema: schema(`{"type":"object","properties":{"suggestion_id":{"type":"string","description":"Leave ` +
			`out to read the suggestion the teammate has open. Give an id (opc_...) only for another one."}}}`),
	}, handle: e.getSuggestion}
}

/*
getSuggestion reads one suggestion the teammate may see.
*/
func (e *Engine) getSuggestion(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "suggestion_id", models.PrefixOperatorChangeset)
	if err != nil && s.viewed == nil {
		return result{}, fmt.Errorf("%w: there is no suggestion with that id; list them with "+
			"list_recent_suggestions", errBadArguments)
	}
	if err != nil {
		id = s.viewed.ID
	}
	var c models.OperatorChangeset
	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		if c, err = q.FindOperatorChangeset(ctx, models.FindOperatorChangesetParams{
			WorkspaceID: s.m.Workspace.ID, ID: id,
		}); err != nil {
			return err
		}
		return Visible(ctx, q, s.m, c)
	})
	if errors.Is(err, database.ErrNotFound) {
		return result{}, fmt.Errorf("%w: there is no suggestion with that id; list them with "+
			"list_recent_suggestions", errBadArguments)
	}
	if err != nil {
		return result{}, err
	}
	text := e.suggestionText(ctx, s, c, e.madeBy(ctx, s, c))
	st := Step{Tool: "get_suggestion_by_id", Summary: "Read the suggestion " + c.Title}
	return result{text: text, step: st}, nil
}

/*
describeChange writes what one operation of a suggestion changes: for
an article, the lines it removes and adds and the text as suggested;
for a procedure, the working copy as suggested.
*/
func (e *Engine) describeChange(ctx context.Context, s *session, b *strings.Builder, op Operation) {
	if op.Kind == KindArticleEdit && op.Article != nil && op.Body != nil {
		if a, err := e.Services.Articles.Show(ctx, s.m, *op.Article); err == nil {
			writeLines(b, "Lines it removes:", changedLines(a.BodyMarkdown, *op.Body))
			writeLines(b, "Lines it adds:", changedLines(*op.Body, a.BodyMarkdown))
		}
		fmt.Fprintf(b, "The article as suggested (article %s):\n%s\n",
			models.PublicID(models.PrefixArticle, *op.Article), truncate(*op.Body, 20000))
		return
	}
	if op.Body != nil {
		fmt.Fprintf(b, "Text:\n%s\n", truncate(*op.Body, 20000))
	}
	if len(op.Definition) > 0 {
		fmt.Fprintf(b, "As suggested:\n%s\n", truncate(string(op.Definition), 20000))
	}
}

/*
writeLines lists lines under a heading, nothing when there are none.
*/
func writeLines(b *strings.Builder, heading string, lines map[string]bool) {
	if len(lines) == 0 {
		return
	}
	b.WriteString(heading + "\n")
	for line := range lines {
		b.WriteString("  " + line + "\n")
	}
}
