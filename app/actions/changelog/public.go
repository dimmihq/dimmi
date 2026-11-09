/*
|--------------------------------------------------------------------------
| What Readers See
|--------------------------------------------------------------------------
|
| The public changelog shows only published entries, as they were last
| published, newest first: never a draft, a scheduled entry or a working
| copy. Readers react with a few fixed emoji, each counted once per
| reader, who is known only by a keyed hash of what identifies them.
|
*/

package changelog

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Emoji returns the reactions readers may leave, by name, in the order
pages show them.
*/
func Emoji() []string {
	return []string{"thumbsup", "heart", "tada", "rocket", "eyes"}
}

/*
ErrEmojiInvalid refuses a reaction that is not one of Emoji.
*/
var ErrEmojiInvalid = fault.New(fault.Invalid, "emoji_invalid", "Pick one of the reactions shown.")

/*
Public reads changelogs for readers. Key keys the hash readers are
known by.
*/
type Public struct {
	DB    *database.DB
	Clock clock.Clock
	Key   []byte
}

/*
Published is an entry as readers see it, with its reaction counts.
*/
type Published struct {
	Entry
	Reactions map[string]int32
}

/*
LabelCount is a label with how many published entries it holds.
*/
type LabelCount struct {
	models.ChangelogLabel
	Entries int32
}

/*
Page is a page of a public changelog: its entries, every label with its
count, the label shown, whether older entries follow, and the dates of
the year's entries for the release strip.
*/
type Page struct {
	Entries []Published
	Labels  []LabelCount
	Label   *models.ChangelogLabel
	More    bool
	Year    int
	Dates   []models.ListChangelogPublishDatesRow
}

/*
PageRequest asks for a page of a help center's changelog, narrowed to
the label with the slug when given.
*/
type PageRequest struct {
	Center models.HelpCenter
	Label  string
	Page   int
	Size   int
}

/*
Index returns a page of the changelog. Found is false for a label that
does not exist or a page past the end.
*/
func (p *Public) Index(ctx context.Context, in PageRequest) (Page, bool, error) {
	var (
		out   Page
		found bool
	)
	err := p.read(ctx, in.Center, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, found, err = p.readPage(ctx, q, in)
		return err
	})
	return out, found, err
}

func (p *Public) readPage(ctx context.Context, q *models.Queries, in PageRequest) (Page, bool, error) {
	ws, hc := in.Center.WorkspaceID, in.Center.ID
	var out Page
	labels, err := p.labels(ctx, q, in.Center)
	if err != nil {
		return out, false, err
	}
	out.Labels = labels
	var labelID *ids.UUID
	if in.Label != "" {
		i := slices.IndexFunc(labels, func(l LabelCount) bool { return l.Slug == in.Label })
		if i < 0 {
			return out, false, nil
		}
		out.Label, labelID = &labels[i].ChangelogLabel, &labels[i].ID
	}
	rows, err := q.ListPublishedChangelogEntries(ctx, models.ListPublishedChangelogEntriesParams{
		WorkspaceID: ws, HelpCenterID: hc, LabelID: labelID,
		RowLimit: int32Of(in.Size + 1), RowOffset: int32Of((in.Page - 1) * in.Size),
	})
	if err != nil {
		return out, false, fmt.Errorf("list published changelog entries: %w", err)
	}
	if len(rows) == 0 && in.Page > 1 {
		return out, false, nil
	}
	out.More = len(rows) > in.Size
	rows = rows[:min(len(rows), in.Size)]
	if out.Entries, err = p.published(ctx, q, ws, rows); err != nil {
		return out, false, err
	}
	out.Year = p.Clock.Now().UTC().Year()
	out.Dates, err = p.dates(ctx, q, in.Center)
	return out, true, err
}

/*
labels lists the changelog's labels with their counts of published
entries.
*/
func (p *Public) labels(ctx context.Context, q *models.Queries, h models.HelpCenter) ([]LabelCount, error) {
	rows, err := q.ListChangelogLabels(ctx, models.ListChangelogLabelsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list changelog labels: %w", err)
	}
	counts, err := q.CountPublishedChangelogLabels(ctx, models.CountPublishedChangelogLabelsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("count changelog labels: %w", err)
	}
	byLabel := make(map[ids.UUID]int32, len(counts))
	for _, c := range counts {
		byLabel[c.LabelID] = c.Entries
	}
	out := make([]LabelCount, len(rows))
	for i, r := range rows {
		out[i] = LabelCount{ChangelogLabel: r, Entries: byLabel[r.ID]}
	}
	return out, nil
}

func (p *Public) dates(ctx context.Context, q *models.Queries, h models.HelpCenter,
) ([]models.ListChangelogPublishDatesRow, error) {
	now := p.Clock.Now().UTC()
	since := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	out, err := q.ListChangelogPublishDates(ctx, models.ListChangelogPublishDatesParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Since: &since, Until: new(since.AddDate(1, 0, 0)),
	})
	if err != nil {
		return nil, fmt.Errorf("list changelog dates: %w", err)
	}
	return out, nil
}

/*
published dresses rows with their labels, authors and reaction counts.
*/
func (p *Public) published(ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.ChangelogEntry,
) ([]Published, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dressed, err := dressAll(ctx, q, ws, refreshed(rows))
	if err != nil {
		return nil, err
	}
	entryIDs := make([]ids.UUID, len(rows))
	for i, r := range rows {
		entryIDs[i] = r.ID
	}
	counts, err := q.CountChangelogReactions(ctx, models.CountChangelogReactionsParams{
		WorkspaceID: ws, EntryIds: entryIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("count changelog reactions: %w", err)
	}
	out := make([]Published, len(dressed))
	at := make(map[ids.UUID]int, len(dressed))
	for i, d := range dressed {
		out[i], at[d.ID] = Published{Entry: d, Reactions: map[string]int32{}}, i
	}
	for _, c := range counts {
		out[at[c.EntryID]].Reactions[c.Emoji] = c.Reactions
	}
	return out, nil
}

/*
Latest returns the newest published entries, for the help center's
home and its header's link to the changelog.
*/
func (p *Public) Latest(ctx context.Context, h models.HelpCenter, n int) ([]Published, error) {
	var out []Published
	err := p.read(ctx, h, func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListPublishedChangelogEntries(ctx, models.ListPublishedChangelogEntriesParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, RowLimit: int32Of(n),
		})
		if err != nil {
			return fmt.Errorf("list latest changelog entries: %w", err)
		}
		out, err = p.published(ctx, q, h.WorkspaceID, rows)
		return err
	})
	return out, err
}

/*
read runs reads inside the help center's workspace.
*/
func (p *Public) read(ctx context.Context, h models.HelpCenter, fn func(context.Context, *models.Queries) error) error {
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	return p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
absolute points the site-relative links and pictures of rendered HTML at
the help center, for mail and feeds read away from the site.
*/
func absolute(html, root string) string {
	return strings.NewReplacer(`src="/`, `src="`+root+`/`, `href="/`, `href="`+root+`/`).Replace(html)
}

/*
int32Of bounds a count to what a query takes.
*/
func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}
