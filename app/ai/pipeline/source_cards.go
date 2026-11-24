/*
|--------------------------------------------------------------------------
| Source Cards
|--------------------------------------------------------------------------
|
| The messenger shows each article an answer cites as a card under the
| reply: its title, a line or two of what it says, and, for a published
| public help article, the slug that opens it in place. A help article's
| own description makes the best line; any other public source gives the
| start of the passage the answer drew on. Agent-only text never reaches
| a card.
|
*/

package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/models"
)

/*
excerptLength is how many characters of a source a card shows at most,
about two lines in the messenger.
*/
const excerptLength = 160

/*
sourceExcerpt is the start of a passage as one line of plain text, cut
at a word near excerptLength.
*/
func sourceExcerpt(text string) string {
	plain := strings.Join(strings.Fields(markdown.PlainText([]byte(text))), " ")
	runes := []rune(plain)
	if len(runes) <= excerptLength {
		return plain
	}

	cut := string(runes[:excerptLength])
	if i := strings.LastIndexByte(cut, ' '); i > excerptLength/2 {
		cut = cut[:i]
	}

	return strings.TrimRight(cut, " ,.;:-") + "…"
}

/*
describeCitations gives each cited help article that readers may open
its slug, and its description as the excerpt when it has one.
*/
func describeCitations(ctx context.Context, tx *database.Tx, workspaceID ids.UUID, cited []citation) error {
	var articles []ids.UUID
	for _, c := range cited {
		if c.articleID != nil {
			articles = append(articles, *c.articleID)
		}
	}
	if len(articles) == 0 {
		return nil
	}

	rows, err := models.New(tx).ListPublicArticleCards(ctx, models.ListPublicArticleCardsParams{
		WorkspaceID: workspaceID, Ids: articles,
	})
	if err != nil {
		return fmt.Errorf("list cited articles: %w", err)
	}

	cards := make(map[ids.UUID]models.ListPublicArticleCardsRow, len(rows))
	for _, row := range rows {
		cards[row.ID] = row
	}
	for i, c := range cited {
		if c.articleID != nil {
			cited[i] = describedBy(c, cards)
		}
	}

	return nil
}

/*
describedBy is the citation with its article's slug and description,
unchanged when the article is not one readers may open.
*/
func describedBy(c citation, cards map[ids.UUID]models.ListPublicArticleCardsRow) citation {
	row, ok := cards[*c.articleID]
	if !ok {
		return c
	}
	c.Slug = row.Slug
	if row.Description != nil && strings.TrimSpace(*row.Description) != "" {
		c.Excerpt = strings.TrimSpace(*row.Description)
	}

	return c
}
