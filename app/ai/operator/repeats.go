/*
|--------------------------------------------------------------------------
| Repeated Suggestions
|--------------------------------------------------------------------------
|
| A routine must not suggest again what already waits for review or what
| the team declined. A key the model makes up cannot tell: it words the
| key differently every time. So an article edit is compared with the
| suggestions about the same article by what it changes: when both
| rewrite the same line of the article, the new one is a repeat, however
| it is titled or keyed.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
repeated says when an article edit of a routine changes a line that a
waiting suggestion, or one declined lately, already changes.
*/
func (e *Engine) repeated(ctx context.Context, s *session, ops []Operation) (string, bool, error) {
	for _, op := range ops {
		if op.Kind != KindArticleEdit || op.Article == nil || op.Body == nil {
			continue
		}
		lines := changedLines(op.Current, *op.Body)
		if len(lines) == 0 {
			continue
		}
		var rows []models.OperatorChangeset
		err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error
			rows, err = models.New(tx).ListOperatorArticleSuggestions(ctx, models.ListOperatorArticleSuggestionsParams{
				WorkspaceID: s.m.Workspace.ID, ArticleID: op.Article.String(), RoutineRunID: &s.routine.run.ID,
				DeclinedSince: e.Clock.Now().Add(-declinedMemory),
			})
			return err
		})
		if err != nil {
			return "", false, fmt.Errorf("list suggestions about the article: %w", err)
		}
		for _, row := range rows {
			if sameLines(row, op, lines) {
				return repeatText(row), true, nil
			}
		}
	}
	return "", false, nil
}

/*
sameLines reports whether a stored suggestion changes one of the lines
an edit of the same article changes.
*/
func sameLines(row models.OperatorChangeset, op Operation, lines map[string]bool) bool {
	var earlier []Operation
	if json.Unmarshal(row.Operations, &earlier) != nil {
		return false
	}
	for _, was := range earlier {
		if was.Kind != KindArticleEdit || was.Article == nil || *was.Article != *op.Article || was.Body == nil {
			continue
		}
		for line := range changedLines(op.Current, *was.Body) {
			if lines[line] {
				return true
			}
		}
	}
	return false
}

/*
repeatText tells the model why its suggestion was not made again.
*/
func repeatText(row models.OperatorChangeset) string {
	if row.Status == StatusPending {
		return fmt.Sprintf("Not suggested again: the suggestion %q already changes this part of the article and "+
			"still waits for review. Do not count it as a suggestion of this run.", row.Title)
	}
	return fmt.Sprintf("Not suggested again: the team declined the suggestion %q, which changed this part of the "+
		"article, on %s. Do not count it as a suggestion of this run.", row.Title,
		row.UpdatedAt.UTC().Format("2006-01-02"))
}

/*
changedLines are the lines of an article that a new body no longer
holds: what an edit rewrites or removes.
*/
func changedLines(current, body string) map[string]bool {
	kept := map[string]bool{}
	for line := range strings.SplitSeq(body, "\n") {
		kept[strings.TrimSpace(line)] = true
	}
	out := map[string]bool{}
	for line := range strings.SplitSeq(current, "\n") {
		if line = strings.TrimSpace(line); line != "" && !kept[line] {
			out[line] = true
		}
	}
	return out
}
