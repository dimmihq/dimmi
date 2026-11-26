/*
|--------------------------------------------------------------------------
| Translation States
|--------------------------------------------------------------------------
|
| Where every article of a help center stands in each language it offers
| besides its default: missing, a draft or published, and whether the
| source moved on since. Editors read it to find what is left to
| translate.
|
*/

package helpcenters

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TranslationState is where one article stands in one language: missing,
draft or published, and whether it is outdated.
*/
type TranslationState struct {
	ArticleID    ids.UUID
	Title        string
	Status       string
	CollectionID *ids.UUID
	Locale       string
	State        string
	Outdated     bool
}

/*
States returns, for every article of the help center that is not
archived, where it stands in each language the help center offers
besides its default, the articles last changed first.
*/
func (t *Translations) States(ctx context.Context, m authz.Membership, helpCenterID ids.UUID,
) ([]TranslationState, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []TranslationState
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenterID)
		if err != nil {
			return err
		}
		rows, err := q.ListHelpCenterTranslationStates(ctx, models.ListHelpCenterTranslationStatesParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: h.ID,
		})
		if err != nil {
			return fmt.Errorf("list translation states: %w", err)
		}
		out = statesOf(rows, LocalesOf(h)[1:])
		return nil
	})
	return out, err
}

/*
statesOf turns each article's translation rows into one state per
language offered.
*/
func statesOf(rows []models.ListHelpCenterTranslationStatesRow, locales []string) []TranslationState {
	var out []TranslationState
	for i := 0; i < len(rows); {
		a := rows[i]
		have := map[string]models.ListHelpCenterTranslationStatesRow{}
		for ; i < len(rows) && rows[i].ID == a.ID; i++ {
			if rows[i].Locale != nil {
				have[*rows[i].Locale] = rows[i]
			}
		}
		for _, l := range locales {
			out = append(out, stateIn(a, have, l))
		}
	}
	return out
}

/*
stateIn is where the article stands in one language.
*/
func stateIn(a models.ListHelpCenterTranslationStatesRow, have map[string]models.ListHelpCenterTranslationStatesRow,
	locale string,
) TranslationState {
	s := TranslationState{
		ArticleID: a.ID, Title: a.Title, Status: a.Status, CollectionID: a.CollectionID, Locale: locale,
		State: "missing",
	}
	if r, ok := have[locale]; ok {
		s.State, s.Outdated = deref(r.TranslationStatus), r.Outdated
	}
	return s
}
