/*
|--------------------------------------------------------------------------
| One Entry for Readers
|--------------------------------------------------------------------------
|
| A published entry on its own page, with the entries just newer and
| older, and the reactions readers leave on it.
|
*/

package changelog

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Neighbor is the published entry beside another, for the links between
entries.
*/
type Neighbor struct {
	Slug      string
	Title     string
	Published time.Time
}

/*
Single is one published entry with the entries just newer and older.
*/
type Single struct {
	Published
	Newer *Neighbor
	Older *Neighbor
}

/*
Entry returns the published entry with the slug. Found is false when
there is none.
*/
func (p *Public) Entry(ctx context.Context, h models.HelpCenter, slug string) (Single, bool, error) {
	var (
		out   Single
		found bool
	)
	err := p.read(ctx, h, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindPublishedChangelogEntryBySlug(ctx, models.FindPublishedChangelogEntryBySlugParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Slug: slug,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find published changelog entry: %w", err)
		}
		list, err := p.published(ctx, q, h.WorkspaceID, []models.ChangelogEntry{row})
		if err != nil {
			return err
		}
		out, found = Single{Published: list[0]}, true
		return p.neighbors(ctx, q, &out)
	})
	return out, found, err
}

func (p *Public) neighbors(ctx context.Context, q *models.Queries, s *Single) error {
	rows, err := q.FindPublishedChangelogNeighbors(ctx, models.FindPublishedChangelogNeighborsParams{
		WorkspaceID: s.WorkspaceID, HelpCenterID: s.HelpCenterID, PublishedAt: *s.PublishedAt, ID: s.ID,
	})
	if err != nil {
		return fmt.Errorf("find changelog neighbors: %w", err)
	}
	for _, r := range rows {
		n := &Neighbor{Slug: r.Slug, Title: deref(r.PublishedTitle)}
		if r.PublishedAt != nil {
			n.Published = *r.PublishedAt
		}
		if r.Side == "newer" {
			s.Newer = n
			continue
		}
		s.Older = n
	}
	return nil
}

/*
Reaction is one reader's emoji on a published entry; Reactor is what
identifies the reader, kept only as a keyed hash.
*/
type Reaction struct {
	Center  models.HelpCenter
	Slug    string
	Emoji   string
	Reactor string
}

/*
React counts the reader's emoji on the entry, once. Found is false when
the entry is not published.
*/
func (p *Public) React(ctx context.Context, in Reaction) (bool, error) {
	if !slices.Contains(Emoji(), in.Emoji) {
		return false, ErrEmojiInvalid
	}
	found := false
	err := p.read(ctx, in.Center, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindPublishedChangelogEntryBySlug(ctx, models.FindPublishedChangelogEntryBySlugParams{
			WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Slug: in.Slug,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find published changelog entry: %w", err)
		}
		found = true
		mac := hashing.HMAC(p.Key, []byte(row.ID.String()+"|"+in.Reactor))
		return q.AddChangelogReaction(ctx, models.AddChangelogReactionParams{
			EntryID: row.ID, WorkspaceID: row.WorkspaceID, ReactorHash: hex.EncodeToString(mac),
			Emoji: in.Emoji, Now: p.Clock.Now(),
		})
	})
	return found, err
}
