/*
|--------------------------------------------------------------------------
| What Readers See
|--------------------------------------------------------------------------
|
| A reader's request names a help center by its subdomain before anyone
| knows the workspace, so that one lookup runs through a security
| definer function; everything after reads inside the workspace. A help
| center's published articles and collections are read once per cache
| version and kept in memory, so pages are served without a query until
| something a reader could see changes. Only published public articles
| are ever in it: drafts, archived and agent-only articles never are.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/memo"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Snapshot is a help center as readers of one language see it at one
cache version. Locale is that language. Collections and Articles carry
that language's names, slugs and text: in the default language the
source, in another what is translated into it and, when the team shows
them, the articles that are not, in the default language; Originals
marks those. Alternates gives,
for each article and collection, its slug in every language readers
may read it in. Restricted says audiences keep some of it from this
reader, so signing in could show more. Messenger is the messenger its
pages load, the zero value when they show none.
*/
type Snapshot struct {
	HelpCenter  models.HelpCenter
	Locale      string
	Collections []models.Collection
	Articles    []models.ListPublishedArticlesRow
	Originals   map[ids.UUID]bool
	Alternates  Alternates
	Restricted  bool
	Messenger   Messenger
}

/*
snapshotKey names a help center at one cache version.
*/
type snapshotKey struct {
	id      ids.UUID
	version int64
	locale  string
}

/*
Public reads help centers for readers who are not signed in: articles
and collections kept to an audience such a reader is not in are left
out of everything they see, search included.
*/
type Public struct {
	DB        *database.DB
	Clock     clock.Clock
	snapshots *memo.Cache[snapshotKey, Snapshot]
}

/*
NewPublic returns a reader keeping up to a thousand help centers for a
minute after their last change was read.
*/
func NewPublic(db *database.DB, clk clock.Clock) *Public {
	return &Public{
		DB: db, Clock: clk, snapshots: memo.New[snapshotKey, Snapshot](memo.Options{Size: 1000, TTL: time.Minute}),
	}
}

/*
ErrNoHelpCenter says no active help center answers the subdomain.
*/
var ErrNoHelpCenter = errors.New("help center not found")

/*
Load returns the help center answering the subdomain as readers of its
default language see it.
*/
func (p *Public) Load(ctx context.Context, subdomain string) (Snapshot, error) {
	return p.LoadIn(ctx, subdomain, "")
}

/*
LoadIn returns the help center answering the subdomain as readers of
the language see it, its default for an empty one or one it does not
offer.
*/
func (p *Public) LoadIn(ctx context.Context, subdomain, locale string) (Snapshot, error) {
	row, err := models.New(p.DB).ResolveHelpCenter(ctx, subdomain)
	if errors.Is(err, database.ErrNotFound) {
		return Snapshot{}, ErrNoHelpCenter
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve help center: %w", err)
	}
	var out Snapshot
	scoped := tenancy.With(ctx, row.WorkspaceID.String())
	err = p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams(row))
		if err != nil {
			return fmt.Errorf("find help center: %w", err)
		}
		key := snapshotKey{id: h.ID, version: h.CacheVersion, locale: readLocale(h, locale)}
		if cached, ok := p.snapshots.Get(key); ok {
			out = cached
			return nil
		}
		out, err = load(ctx, q, h, reading{locale: key.locale, now: p.Clock.Now})
		if err == nil {
			p.snapshots.Set(key, out)
		}
		return err
	})
	return out, err
}

/*
readLocale is the language a snapshot is read in: the one asked for
when the help center offers it, else its default.
*/
func readLocale(h models.HelpCenter, locale string) string {
	if locale != "" && OffersLocale(h, locale) {
		return locale
	}
	return h.Locale
}

/*
LoadFor returns the help center answering the subdomain as one signed-in
reader sees it, with what their audiences open. It is never cached, so
nothing one reader sees reaches another.
*/
func (p *Public) LoadFor(ctx context.Context, subdomain string, contact ids.UUID) (Snapshot, error) {
	return p.LoadForIn(ctx, subdomain, "", contact)
}

/*
LoadForIn is LoadFor in one language.
*/
func (p *Public) LoadForIn(ctx context.Context, subdomain, locale string, contact ids.UUID) (Snapshot, error) {
	row, err := models.New(p.DB).ResolveHelpCenter(ctx, subdomain)
	if errors.Is(err, database.ErrNotFound) {
		return Snapshot{}, ErrNoHelpCenter
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve help center: %w", err)
	}

	return p.readFor(ctx, models.FindHelpCenterParams(row), locale, contact)
}

/*
LoadByIDFor returns a workspace's help center as one reader the
messenger knows sees it, for the messenger's Help tab. It is never
cached either.
*/
func (p *Public) LoadByIDFor(ctx context.Context, workspaceID, id, contact ids.UUID) (Snapshot, error) {
	return p.LoadByIDForIn(ctx, models.FindHelpCenterParams{WorkspaceID: workspaceID, ID: id}, "", contact)
}

/*
LoadByIDForIn is LoadByIDFor in one language.
*/
func (p *Public) LoadByIDForIn(ctx context.Context, find models.FindHelpCenterParams, locale string, contact ids.UUID,
) (Snapshot, error) {
	return p.readFor(ctx, find, locale, contact)
}

/*
readFor reads the help center as the reader sees it, inside its
workspace.
*/
func (p *Public) readFor(ctx context.Context, find models.FindHelpCenterParams, locale string, contact ids.UUID,
) (Snapshot, error) {
	var out Snapshot
	scoped := tenancy.With(ctx, find.WorkspaceID.String())
	err := p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := q.FindHelpCenter(ctx, find)
		if errors.Is(err, database.ErrNotFound) || (err == nil && h.Status != "active") {
			return ErrNoHelpCenter
		}
		if err != nil {
			return fmt.Errorf("find help center: %w", err)
		}
		reader, err := p.reader(ctx, q, h.WorkspaceID, &contact)
		if err != nil {
			return err
		}
		full, err := loadAll(ctx, q, h, readLocale(h, locale))
		if err != nil {
			return err
		}
		out, err = readable(ctx, q, full, reader)
		return err
	})
	return out, err
}

/*
reader is who reads: a contact, or nil for a reader nobody knows.
*/
func (p *Public) reader(ctx context.Context, q *models.Queries, ws ids.UUID, contact *ids.UUID,
) (audiences.Reader, error) {
	out := audiences.Reader{WorkspaceID: ws, Now: p.Clock.Now()}
	if contact == nil {
		return out, nil
	}
	c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *contact})
	if err != nil {
		return out, fmt.Errorf("find reader: %w", err)
	}
	people, err := audiences.LoadPeople(ctx, q, ws, []models.Contact{c})
	if err != nil {
		return out, err
	}
	out.Person = &people[0]
	return out, nil
}

/*
load reads a help center's collections and published articles, those an
anonymous reader may see.
*/
func load(ctx context.Context, q *models.Queries, h models.HelpCenter, at reading) (Snapshot, error) {
	out, err := loadAll(ctx, q, h, at.locale)
	if err != nil {
		return out, err
	}
	return readable(ctx, q, out, audiences.Reader{WorkspaceID: h.WorkspaceID, Now: at.now()})
}

/*
loadAll reads every collection and published article of a help center
in the language, before any audience is applied.
*/
func loadAll(ctx context.Context, q *models.Queries, h models.HelpCenter, locale string) (Snapshot, error) {
	out := Snapshot{HelpCenter: h, Locale: locale}
	read, err := readShelves(ctx, q, h)
	if err != nil {
		return out, err
	}
	shown := read.in(locale, SettingsOf(h).UntranslatedArticles)
	out.Collections, out.Articles, out.Originals = shown.collections, shown.articles, shown.originals
	out.Alternates = read.alternates()
	out.Messenger, err = messengerOf(ctx, q, h)
	return out, err
}

/*
nowFunc gives the current time.
*/
type nowFunc func() time.Time

/*
reading is the language a snapshot is read in, and the clock its
audiences are judged by.
*/
type reading struct {
	locale string
	now    nowFunc
}

/*
Find is one search of a help center: the help center, the words, how
many results at most, and the signed-in reader, nil for anyone else.
*/
type Find struct {
	Center models.HelpCenter
	Locale string
	Query  string
	Limit  int32
	Reader *ids.UUID
}

/*
Search finds a help center's published public articles matching the
query, best first, those the reader may read.
*/
func (p *Public) Search(ctx context.Context, f Find) ([]models.SearchPublishedArticlesRow, error) {
	var out []models.SearchPublishedArticlesRow
	h := f.Center
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	err := p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		rows, err := searchIn(ctx, q, f)
		if err != nil {
			return err
		}
		visible, err := p.articlesFor(ctx, q, h.WorkspaceID, f.Reader)
		for _, r := range rows {
			if err == nil && visible(r.ID) {
				out = append(out, r)
			}
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("search articles: %w", err)
	}
	return out, nil
}

/*
Redirect returns the path an old address of the help center now leads
to, relative to the help center's root. A redirect of another language
leads to its target in that language.
*/
func (p *Public) Redirect(ctx context.Context, h models.HelpCenter, from string) (string, bool) {
	var to string
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	_ = p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).FindLocalizedRedirect(ctx, models.FindLocalizedRedirectParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, FromPath: from,
		})
		if err != nil {
			return err
		}
		prefix := ""
		if row.TargetLocale != nil && *row.TargetLocale != h.Locale {
			prefix = "/" + *row.TargetLocale
		}
		switch {
		case row.ArticleSlug != "":
			to = prefix + "/articles/" + row.ArticleSlug
		case row.CollectionSlug != "":
			to = prefix + "/collections/" + row.CollectionSlug
		case row.TargetUrl != nil:
			to = *row.TargetUrl
		}
		return nil
	})
	return to, to != ""
}

/*
LoadByID returns a workspace's help center as readers see it, for the
messenger, which knows its help center by id.
*/
func (p *Public) LoadByID(ctx context.Context, workspaceID, id ids.UUID) (Snapshot, error) {
	return p.LoadByIDIn(ctx, workspaceID, id, "")
}

/*
LoadByIDIn is LoadByID in one language.
*/
func (p *Public) LoadByIDIn(ctx context.Context, workspaceID, id ids.UUID, locale string) (Snapshot, error) {
	var out Snapshot
	scoped := tenancy.With(ctx, workspaceID.String())
	err := p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: workspaceID, ID: id})
		if errors.Is(err, database.ErrNotFound) || (err == nil && h.Status != "active") {
			return ErrNoHelpCenter
		}
		if err != nil {
			return fmt.Errorf("find help center: %w", err)
		}
		key := snapshotKey{id: h.ID, version: h.CacheVersion, locale: readLocale(h, locale)}
		if cached, ok := p.snapshots.Get(key); ok {
			out = cached
			return nil
		}
		out, err = load(ctx, q, h, reading{locale: key.locale, now: p.Clock.Now})
		if err == nil {
			p.snapshots.Set(key, out)
		}
		return err
	})
	return out, err
}

/*
articlesFor answers whether the reader, nil for anyone, may read an
article.
*/
func (p *Public) articlesFor(ctx context.Context, q *models.Queries, ws ids.UUID, contact *ids.UUID,
) (func(ids.UUID) bool, error) {
	r, err := audiences.LoadRestrictions(ctx, q, ws)
	if err != nil {
		return nil, err
	}

	reader, err := p.reader(ctx, q, ws, contact)
	if err != nil {
		return nil, err
	}

	reader.Needed = r.Needed()
	allowed, err := audiences.Allowed(ctx, q, reader)

	return func(id ids.UUID) bool { return r.Article(id, allowed) }, err
}
