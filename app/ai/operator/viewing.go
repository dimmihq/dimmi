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
lookAt reads what the teammate has open and returns the note the model
reads with their question, empty when nothing is open. The suggestion
is kept on the session only while it waits and the teammate may see it.
*/
func (e *Engine) lookAt(ctx context.Context, s *session) string {
	v := s.viewing
	var open []string
	if v.Article != nil {
		if a, err := e.Services.Articles.Show(ctx, s.m, *v.Article); err == nil {
			open = append(open, fmt.Sprintf("the help article %q (%s)", a.Title,
				models.PublicID(models.PrefixArticle, a.ID)))
		}
	}
	if v.Procedure != nil {
		if p, err := e.Services.Procedures.Show(ctx, s.m, *v.Procedure); err == nil {
			open = append(open, fmt.Sprintf("the procedure %q (%s)", p.Name,
				models.PublicID(models.PrefixProcedure, p.ID)))
		}
	}
	if len(open) == 0 {
		return ""
	}
	note := "[The teammate has this open: " + strings.Join(open, " and ") + ". A change they ask for is about it " +
		"unless they say otherwise: read it, then make the change with propose_article_edits for an article or " +
		"propose_changes for a procedure. It shows on their page for them to accept."
	if c, ok := e.viewedSuggestion(ctx, s); ok {
		s.viewed = &c
		note += fmt.Sprintf(" A suggestion waits for review on it: %q (%s). Read it with get_suggestion_by_id "+
			"before you explain it or change it. Changes you make to this article or procedure are added to "+
			"that suggestion, so quote passages from the text it shows as suggested, not from the article as "+
			"published.", c.Title, models.PublicID(models.PrefixOperatorChangeset, c.ID))
	}
	return note + "]"
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
			"the lines it removes and adds with the article's full text as suggested.",
		Schema: schema(`{"type":"object","required":["suggestion_id"],"properties":{"suggestion_id":{` +
			`"type":"string","description":"The suggestion's id, such as opc_..."}}}`),
	}, handle: e.getSuggestion}
}

/*
getSuggestion reads one suggestion the teammate may see.
*/
func (e *Engine) getSuggestion(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "suggestion_id", models.PrefixOperatorChangeset)
	if err != nil {
		return result{}, err
	}
	var c models.OperatorChangeset
	made := "in a chat with Operator"
	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		if c, err = q.FindOperatorChangeset(ctx, models.FindOperatorChangesetParams{
			WorkspaceID: s.m.Workspace.ID, ID: id,
		}); err != nil {
			return err
		}
		if err := Visible(ctx, q, s.m, c); err != nil || c.RoutineID == nil {
			return err
		}
		routine, err := q.FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{
			WorkspaceID: s.m.Workspace.ID, ID: *c.RoutineID,
		})
		if err == nil {
			made = fmt.Sprintf("by the routine %q", routine.Name)
		}
		return nil
	})
	if errors.Is(err, database.ErrNotFound) {
		return result{}, fmt.Errorf("%w: there is no suggestion with that id", errBadArguments)
	}
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Suggestion %q (%s)\nStatus: %s\nMade %s on %s\nReason: %s\n", c.Title,
		models.PublicID(models.PrefixOperatorChangeset, c.ID), c.Status, made,
		c.CreatedAt.UTC().Format("2006-01-02"), c.Reason)
	for _, op := range Operations(c.Operations) {
		fmt.Fprintf(&b, "\n%s\n", op.Label)
		e.describeChange(ctx, s, &b, op)
	}
	st := Step{Tool: "get_suggestion_by_id", Summary: "Read the suggestion " + c.Title}
	return result{text: b.String(), step: st}, nil
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
