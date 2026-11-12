/*
|--------------------------------------------------------------------------
| Widget Help Controller
|--------------------------------------------------------------------------
|
| The messenger's Help tab: the collections and published public
| articles of the help center the messenger links to, with each body as
| plain blocks, and the help center's address. Anyone gets what anyone may
| read, so a page can show help before the visitor says anything. A
| visitor the messenger knows, signed in by the team's product or with a
| proven email, gets what their audiences open, and a pass that opens the
| help center with them already signed in.
|
*/

package widget

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxHelpArticles caps the articles the Help tab carries at once.
*/
const maxHelpArticles = 200

/*
HelpController serves the messenger's Help tab. Readers knows who a
visitor reads as and issues passes; Addresses builds the help center's
address; Ratings keeps what visitors say about articles.
*/
type HelpController struct {
	Public    *helpcenters.Public
	Readers   *helpcenters.Readers
	Ratings   *helpcenters.Ratings
	Addresses helpcenters.Addresses
	Clock     clock.Clock
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
HelpIndex is everything the Help tab draws, and the address the help
center's pages start at, empty without one.
*/
type HelpIndex struct {
	URL         string           `json:"url"`
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
	resolved, _ := actions.CurrentResolved(ctx)

	return c.answer(ctx, resolved, nil)
}

/*
Mine answers the Help tab as the visitor reads it: what their audiences
open when the messenger knows them, what anyone may read otherwise.
*/
func (c *HelpController) Mine(ctx context.Context, _ *requests.NoInput) (*HelpOutput, error) {
	auth := authenticated(ctx)
	var contact *ids.UUID
	if r, ok := c.known(auth); ok {
		contact = r.Contact
		if contact == nil {
			contact = c.readsAs(ctx, auth, r.Email)
		}
	}

	return c.answer(ctx, auth.Resolved, contact)
}

/*
answer reads the messenger's help center for the reader, nil for anyone.
*/
func (c *HelpController) answer(ctx context.Context, resolved actions.Resolved, contact *ids.UUID,
) (*HelpOutput, error) {
	out := &HelpOutput{}
	out.Body.Data = HelpIndex{Collections: []HelpCollection{}, Articles: []HelpArticle{}, Popular: []string{}}
	if resolved.Widget.HelpCenterID == nil || c.Public == nil {
		return out, nil
	}

	ws, id := resolved.Workspace.ID, *resolved.Widget.HelpCenterID
	snap, err := c.Public.LoadByID(ctx, ws, id)
	if err == nil && contact != nil {
		snap, err = c.Public.LoadByIDFor(ctx, ws, id, *contact)
	}

	if errors.Is(err, helpcenters.ErrNoHelpCenter) {
		return out, nil
	}

	if err != nil {
		return nil, err
	}

	out.Body.Data = indexOf(snap)
	out.Body.Data.URL = c.Addresses.Root(snap.HelpCenter)

	return out, nil
}

/*
known says who the messenger knows the visitor as: the contact the
team's product signed in, or the email they proved, false for anyone
else.
*/
func (c *HelpController) known(auth actions.Authenticated) (helpcenters.PassFor, bool) {
	s := auth.Session
	if s.Kind == string(enums.WidgetSessionIdentified) {
		return helpcenters.PassFor{Contact: &s.ContactID}, true
	}

	if s.VerifiedEmail != nil && actions.EmailProven(s, c.Clock.Now()) {
		return helpcenters.PassFor{Email: *s.VerifiedEmail}, true
	}

	return helpcenters.PassFor{}, false
}

/*
readsAs is who a visitor with a proven email reads as: the customer the
team's product named with it, else their own lead.
*/
func (c *HelpController) readsAs(ctx context.Context, auth actions.Authenticated, email string) *ids.UUID {
	if found, ok := c.Readers.Known(ctx, auth.Resolved.Workspace.ID, email); ok {
		return &found
	}

	return &auth.Session.ContactID
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
