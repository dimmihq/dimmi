/*
|--------------------------------------------------------------------------
| Changelog Entries
|--------------------------------------------------------------------------
|
| An entry is edited as a working copy and read as its published copy,
| as help center articles are: saving never changes what readers see
| until the next publish. Two editors cannot overwrite each other: a save
| names the lock version it started from and is refused when someone
| saved since. Everyone who saves an entry is kept as one of its editors,
| and the editors are its authors until someone picks the authors by
| hand.
|
*/

package changelog

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing changelog entries.
*/
var (
	ErrEntryNotFound = fault.New(fault.NotFound, "changelog_entry_not_found",
		"No changelog entry of this workspace has this id.")
	ErrEntryChanged = fault.New(fault.Conflict, "changelog_entry_changed",
		"Someone saved this entry since you opened it. Reload to see their changes.")
	ErrTitleInvalid = fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
	ErrSlugInvalid  = fault.New(fault.Invalid, "slug_invalid",
		"Use lowercase letters, digits and single dashes.").OnField("slug")
	ErrSlugTaken = fault.New(fault.Conflict, "slug_taken",
		"Another entry of this changelog already uses this slug.").OnField("slug")
	ErrSummaryTooLong = fault.New(fault.Invalid, "summary_too_long",
		"Summaries are at most 300 characters.").OnField("summary")
	ErrCoverInvalid = fault.New(fault.Invalid, "cover_invalid",
		"Covers are pictures uploaded to the help center.").OnField("cover_url")
	ErrTooManyAuthors = fault.New(fault.Invalid, "too_many_authors",
		"An entry has at most 5 authors.").OnField("author_ids")
)

/*
maxAuthors is how many authors an entry names at most.
*/
const maxAuthors = 5

/*
Entries manages a changelog's entries. Queue holds scheduled publishes
and mailings until their time.
*/
type Entries struct {
	DB       *database.DB
	Clock    clock.Clock
	Queue    *queue.Queue
	Renderer *markdown.Renderer
}

/*
NewEntries returns the entries action with the help center's renderer,
so entries read the same as articles.
*/
func NewEntries(db *database.DB, clk clock.Clock, q *queue.Queue) *Entries {
	return &Entries{DB: db, Clock: clk, Queue: q, Renderer: markdown.New(markdown.Options{
		GFM: true, Footnotes: true, Alerts: true, HeadingIDs: true, ImageLayout: true,
	})}
}

/*
EntryInput is an entry to create or a change to one. On change, nil
leaves a field as it is. LabelIDs and AuthorIDs replace the entry's
labels and chosen authors when set; an empty AuthorIDs goes back to
naming the editors. LockVersion is the version the editor started from.
*/
type EntryInput struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	ID           ids.UUID
	Title        *string
	Slug         *string
	Summary      *string
	Body         *string
	CoverURL     *string
	LabelIDs     *[]ids.UUID
	AuthorIDs    *[]ids.UUID
	LockVersion  int32
}

/*
Show returns one entry.
*/
func (e *Entries) Show(ctx context.Context, m authz.Membership, id ids.UUID) (Entry, error) {
	if !m.Can("changelog:read") {
		return Entry{}, exceptions.ErrForbidden
	}
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := findEntry(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		out, err = dress(ctx, q, m.Workspace.ID, row)
		return err
	})
	return out, err
}

/*
Create makes a draft, written by the member creating it.
*/
func (e *Entries) Create(ctx context.Context, in EntryInput) (Entry, error) {
	if !in.Membership.Can("changelog:write") {
		return Entry{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, e.Clock.Now()
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{
			WorkspaceID: ws, ID: in.HelpCenterID,
		}); err != nil {
			return notFound(err, helpcenters.ErrHelpCenterNotFound)
		}
		w, err := newWorking(in)
		if err != nil {
			return err
		}
		at := slugAt{ws: ws, hc: in.HelpCenterID, slug: w.slug, typed: in.Slug != nil}
		if w.slug, err = freeSlug(ctx, q, at); err != nil {
			return err
		}
		row, err := q.CreateChangelogEntry(ctx, models.CreateChangelogEntryParams{
			WorkspaceID: ws, HelpCenterID: in.HelpCenterID, Title: w.title, Slug: w.slug, Summary: w.summary,
			BodyMarkdown: w.body, CoverUrl: w.cover, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create changelog entry: %w", err)
		}
		if err := e.relate(ctx, q, row, in); err != nil {
			return err
		}
		out, err = dress(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
Update saves the working copy, refused when someone saved since the
editor started.
*/
func (e *Entries) Update(ctx context.Context, in EntryInput) (Entry, error) {
	if !in.Membership.Can("changelog:write") {
		return Entry{}, exceptions.ErrForbidden
	}
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := e.save(ctx, q, in)
		if err != nil {
			return err
		}
		if err := e.relate(ctx, q, row, in); err != nil {
			return err
		}
		out, err = dress(ctx, q, in.Membership.Workspace.ID, row)
		return err
	})
	return out, err
}

/*
save writes the change over the locked working copy, refused when
someone saved since the editor started.
*/
func (e *Entries) save(ctx context.Context, q *models.Queries, in EntryInput) (models.ChangelogEntry, error) {
	ws := in.Membership.Workspace.ID
	current, err := lockEntry(ctx, q, ws, in.ID)
	if err != nil {
		return current, err
	}
	if in.LockVersion != 0 && in.LockVersion != current.LockVersion {
		return current, ErrEntryChanged
	}
	w, err := changedWorking(current, in)
	if err != nil {
		return current, err
	}
	at := slugAt{ws: ws, hc: current.HelpCenterID, self: current.ID, slug: w.slug, typed: in.Slug != nil}
	if w.slug, err = freeSlug(ctx, q, at); err != nil {
		return current, err
	}
	row, err := q.UpdateChangelogEntry(ctx, models.UpdateChangelogEntryParams{
		Title: w.title, Slug: w.slug, Summary: w.summary, BodyMarkdown: w.body, CoverUrl: w.cover,
		AuthorsChosen: chosen(current, in), Now: e.Clock.Now(), WorkspaceID: ws, ID: current.ID,
		LockVersion: current.LockVersion,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, ErrEntryChanged
	}
	if err != nil {
		return row, fmt.Errorf("save changelog entry: %w", err)
	}
	return row, nil
}

/*
chosen says whether the entry names its authors by hand after the
change: picking authors does, clearing them goes back to the editors.
*/
func chosen(current models.ChangelogEntry, in EntryInput) bool {
	if in.AuthorIDs == nil {
		return current.AuthorsChosen
	}
	return len(*in.AuthorIDs) > 0
}

/*
relate keeps the saving member as an editor and replaces the labels and
chosen authors the input names.
*/
func (e *Entries) relate(ctx context.Context, q *models.Queries, row models.ChangelogEntry, in EntryInput) error {
	ws, now := row.WorkspaceID, e.Clock.Now()
	if in.Membership.Member.ID != (ids.UUID{}) {
		if err := q.AddChangelogEditor(ctx, models.AddChangelogEditorParams{
			EntryID: row.ID, MemberID: in.Membership.Member.ID, WorkspaceID: ws, Now: now,
		}); err != nil {
			return fmt.Errorf("keep changelog editor: %w", err)
		}
	}
	if in.LabelIDs != nil {
		if err := setLabels(ctx, q, row, *in.LabelIDs); err != nil {
			return err
		}
	}
	if in.AuthorIDs != nil {
		return setAuthors(ctx, q, row, authorsAt{members: *in.AuthorIDs, now: now})
	}
	return nil
}

/*
working is the editable part of an entry, checked.
*/
type working struct {
	title, slug, body string
	summary, cover    *string
}

func newWorking(in EntryInput) (working, error) {
	w := working{title: strings.TrimSpace(deref(in.Title))}
	if w.title == "" {
		w.title = "Untitled update"
	}
	w.slug = helpcenters.Slugify(w.title, 120)
	if w.slug == "" {
		w.slug = "update"
	}
	err := w.overlay(in)
	return w, err
}

/*
changedWorking is the working copy after the change. Until the entry is
first published, a slug nobody typed follows the title.
*/
func changedWorking(current models.ChangelogEntry, in EntryInput) (working, error) {
	w := working{
		title: current.Title, slug: current.Slug, body: current.BodyMarkdown, summary: current.Summary,
		cover: current.CoverUrl,
	}
	err := w.overlay(in)
	if err != nil || in.Slug != nil || current.PublishedAt != nil || !followsTitle(current) {
		return w, err
	}
	if slug := helpcenters.Slugify(w.title, 120); slug != "" {
		w.slug = slug
	}
	return w, nil
}

/*
followsTitle says whether the entry's slug is still the one its title
made, give or take the number added to keep it free.
*/
func followsTitle(e models.ChangelogEntry) bool {
	made := helpcenters.Slugify(e.Title, 120)
	if made == "" {
		made = "update"
	}
	return e.Slug == made || strings.HasPrefix(e.Slug, made+"-")
}

/*
overlay applies the fields the input sets, checking each.
*/
func (w *working) overlay(in EntryInput) error {
	if in.Title != nil {
		w.title = strings.TrimSpace(*in.Title)
	}
	if w.title == "" || len([]rune(w.title)) > 150 {
		return ErrTitleInvalid
	}
	if in.Slug != nil {
		w.slug = strings.TrimSpace(*in.Slug)
		if !slugPattern.MatchString(w.slug) || len(w.slug) > 120 {
			return ErrSlugInvalid
		}
	}
	if in.Summary != nil {
		w.summary = nonEmpty(strings.TrimSpace(*in.Summary))
	}
	if w.summary != nil && len([]rune(*w.summary)) > 300 {
		return ErrSummaryTooLong
	}
	if in.Body != nil {
		w.body = *in.Body
	}
	if in.CoverURL != nil {
		w.cover = nonEmpty(strings.TrimSpace(*in.CoverURL))
	}
	if w.cover != nil && !strings.HasPrefix(*w.cover, "/help-images/") {
		return ErrCoverInvalid
	}
	return nil
}
