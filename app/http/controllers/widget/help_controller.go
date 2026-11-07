/*
|--------------------------------------------------------------------------
| Widget Help Controller
|--------------------------------------------------------------------------
|
| The messenger's Help tab: the collections and published public
| articles of the help center the messenger links to, with each body as
| plain blocks, and search over them. Nothing here needs a session, so a
| page can show help before the visitor says anything.
|
*/

package widget

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxHelpArticles caps the articles the Help tab carries at once.
*/
const maxHelpArticles = 200

/*
HelpController serves the messenger's Help tab.
*/
type HelpController struct {
	Public *helpcenters.Public
}

/*
HelpArticle is an article as the messenger shows it.
*/
type HelpArticle struct {
	Slug    string              `json:"slug"`
	Title   string              `json:"title"`
	Summary string              `json:"summary"`
	Updated time.Time           `json:"updated"`
	Body    []helpcenters.Block `json:"body"`
}

/*
HelpCollection is a collection as the messenger lists it, with the
slugs of its articles.
*/
type HelpCollection struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Articles    []string `json:"articles"`
}

/*
HelpIndex is everything the Help tab draws.
*/
type HelpIndex struct {
	Collections []HelpCollection `json:"collections"`
	Articles    []HelpArticle    `json:"articles"`
	Popular     []string         `json:"popular"`
}

/*
HelpOutput answers the Help tab.
*/
type HelpOutput struct {
	Body struct {
		Data HelpIndex `json:"data"`
	}
}

/*
Show answers the Help tab of the messenger the request names: empty
when it links to no help center.
*/
func (c *HelpController) Show(ctx context.Context, _ *requests.NoInput) (*HelpOutput, error) {
	out := &HelpOutput{}
	out.Body.Data = HelpIndex{Collections: []HelpCollection{}, Articles: []HelpArticle{}, Popular: []string{}}
	resolved, _ := actions.CurrentResolved(ctx)
	if resolved.Widget.HelpCenterID == nil || c.Public == nil {
		return out, nil
	}
	snap, err := c.Public.LoadByID(ctx, resolved.Workspace.ID, *resolved.Widget.HelpCenterID)
	if errors.Is(err, helpcenters.ErrNoHelpCenter) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out.Body.Data = indexOf(snap)
	return out, nil
}

/*
indexOf shapes a help center for the Help tab.
*/
func indexOf(snap helpcenters.Snapshot) HelpIndex {
	out := HelpIndex{Collections: []HelpCollection{}, Articles: []HelpArticle{}, Popular: []string{}}
	slugs := map[string][]string{}
	for i, a := range snap.Articles {
		if i == maxHelpArticles {
			break
		}
		out.Articles = append(out.Articles, HelpArticle{
			Slug: a.Slug, Title: a.Title, Summary: deref(a.Description), Updated: a.VersionCreatedAt,
			Body: helpcenters.Blocks(a.BodyMarkdown),
		})
		if len(out.Popular) < 5 {
			out.Popular = append(out.Popular, a.Slug)
		}
		if a.CollectionID != nil {
			slugs[a.CollectionID.String()] = append(slugs[a.CollectionID.String()], a.Slug)
		}
	}
	for _, col := range snap.Collections {
		out.Collections = append(out.Collections, collectionOf(col, slugs[col.ID.String()]))
	}
	return out
}

func collectionOf(c models.Collection, articles []string) HelpCollection {
	if articles == nil {
		articles = []string{}
	}
	return HelpCollection{Slug: c.Slug, Title: c.Name, Description: deref(c.Description), Articles: articles}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
