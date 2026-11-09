/*
|--------------------------------------------------------------------------
| Dressed Entries
|--------------------------------------------------------------------------
|
| An entry as the team sees it: its row with the labels it is filed
| under, the teammates who edited it and the authors it shows. While
| nobody picked authors by hand, the editors are the authors, in the
| order they first touched the entry.
|
*/

package changelog

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Person is a teammate named on an entry, with the path of their picture
when they have one.
*/
type Person struct {
	MemberID ids.UUID
	Name     string
	Avatar   *string
}

/*
Label is a label an entry is filed under.
*/
type Label struct {
	ID    ids.UUID
	Name  string
	Slug  string
	Color string
}

/*
Entry is an entry with its labels, editors and authors.
*/
type Entry struct {
	models.ChangelogEntry
	Labels  []Label
	Editors []Person
	Authors []Person
}

/*
slugPattern is what an entry's slug is made of.
*/
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

/*
dress adds an entry's labels and people.
*/
func dress(ctx context.Context, q *models.Queries, ws ids.UUID, row models.ChangelogEntry) (Entry, error) {
	out, err := dressAll(ctx, q, ws, []models.ChangelogEntry{row})
	if err != nil {
		return Entry{}, err
	}
	return out[0], nil
}

/*
dressAll adds the labels and people of every entry in two queries.
*/
func dressAll(ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.ChangelogEntry) ([]Entry, error) {
	entryIDs := make([]ids.UUID, len(rows))
	for i, r := range rows {
		entryIDs[i] = r.ID
	}
	labels, err := q.ListChangelogEntryLabels(ctx, models.ListChangelogEntryLabelsParams{
		WorkspaceID: ws, EntryIds: entryIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list changelog entry labels: %w", err)
	}
	people, err := q.ListChangelogEntryPeople(ctx, models.ListChangelogEntryPeopleParams{
		WorkspaceID: ws, EntryIds: entryIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list changelog entry people: %w", err)
	}
	byID := make(map[ids.UUID]*Entry, len(rows))
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = Entry{ChangelogEntry: r}
		byID[r.ID] = &out[i]
	}
	for _, l := range labels {
		e := byID[l.EntryID]
		e.Labels = append(e.Labels, Label{ID: l.ID, Name: l.Name, Slug: l.Slug, Color: l.Color})
	}
	for _, p := range people {
		e, person := byID[p.EntryID], Person{MemberID: p.MemberID, Name: p.Name, Avatar: p.AvatarPath}
		switch p.Role {
		case "author":
			e.Authors = append(e.Authors, person)
		default:
			e.Editors = append(e.Editors, person)
		}
	}
	for i := range out {
		if !out[i].AuthorsChosen || len(out[i].Authors) == 0 {
			out[i].Authors = out[i].Editors
		}
	}
	return out, nil
}

func findEntry(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.ChangelogEntry, error) {
	out, err := q.FindChangelogEntry(ctx, models.FindChangelogEntryParams{WorkspaceID: ws, ID: id})
	return out, notFound(err, ErrEntryNotFound)
}

func lockEntry(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.ChangelogEntry, error) {
	out, err := q.LockChangelogEntry(ctx, models.LockChangelogEntryParams{WorkspaceID: ws, ID: id})
	return out, notFound(err, ErrEntryNotFound)
}

/*
notFound maps a missing row to the feature's own failure.
*/
func notFound(err, missing error) error {
	if errors.Is(err, database.ErrNotFound) {
		return missing
	}
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	return nil
}

/*
slugAt is where a slug must be free: the help center, apart from the
entry itself.
*/
type slugAt struct {
	ws, hc, self ids.UUID
	slug         string
	typed        bool
}

/*
freeSlug returns the slug when nobody else has it. A slug someone typed
must be free as typed; one made from the title gets a number added
until it is.
*/
func freeSlug(ctx context.Context, q *models.Queries, at slugAt) (string, error) {
	for n := 1; n < 100; n++ {
		candidate := at.slug
		if n > 1 {
			candidate = at.slug + "-" + strconv.Itoa(n)
		}
		taken, err := q.ChangelogSlugTaken(ctx, models.ChangelogSlugTakenParams{
			WorkspaceID: at.ws, HelpCenterID: at.hc, Slug: candidate, ID: at.self,
		})
		if err != nil {
			return "", fmt.Errorf("check changelog slug: %w", err)
		}
		if !taken {
			return candidate, nil
		}
		if at.typed {
			return "", ErrSlugTaken
		}
	}
	return "", ErrSlugTaken
}

/*
setLabels files the entry under exactly the help center labels named.
*/
func setLabels(ctx context.Context, q *models.Queries, row models.ChangelogEntry, labels []ids.UUID) error {
	if err := q.ClearChangelogEntryLabels(ctx, models.ClearChangelogEntryLabelsParams{
		WorkspaceID: row.WorkspaceID, EntryID: row.ID,
	}); err != nil {
		return fmt.Errorf("clear changelog entry labels: %w", err)
	}
	if err := q.AddChangelogEntryLabels(ctx, models.AddChangelogEntryLabelsParams{
		EntryID: row.ID, WorkspaceID: row.WorkspaceID, HelpCenterID: row.HelpCenterID, LabelIds: labels,
	}); err != nil {
		return fmt.Errorf("label changelog entry: %w", err)
	}
	return nil
}

/*
authorsAt is the teammates to name as authors, in order, and when.
*/
type authorsAt struct {
	members []ids.UUID
	now     time.Time
}

/*
setAuthors names exactly the teammates given as the entry's authors, in
order.
*/
func setAuthors(ctx context.Context, q *models.Queries, row models.ChangelogEntry, at authorsAt) error {
	if len(at.members) > maxAuthors {
		return ErrTooManyAuthors
	}
	if err := q.ClearChangelogAuthors(ctx, models.ClearChangelogAuthorsParams{
		WorkspaceID: row.WorkspaceID, EntryID: row.ID,
	}); err != nil {
		return fmt.Errorf("clear changelog authors: %w", err)
	}
	for i, member := range at.members {
		if err := q.AddChangelogAuthor(ctx, models.AddChangelogAuthorParams{
			EntryID: row.ID, MemberID: member, WorkspaceID: row.WorkspaceID, Position: int32Of(i), Now: at.now,
		}); err != nil {
			return fmt.Errorf("name changelog author: %w", err)
		}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
