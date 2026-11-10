/*
|--------------------------------------------------------------------------
| Slack Digests
|--------------------------------------------------------------------------
|
| A Slack channel can get a digest every day at the time it picked, in
| the workspace's zone: the standup lists each teammate's open queue and
| what nobody holds; the summary counts yesterday's conversations, how
| fast the team first answered, how the agent did, and names the one
| that waited longest; on Mondays the themes of last week name what
| customers wrote about most. A digest is scheduled again before it is
| posted, so a failing post never repeats it.
|
| The themes come from a theme finder when one is set, which may read
| the week's subjects with a model; without one, the week's most used
| tags stand in for themes.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The digests a Slack channel can get.
*/
const (
	DigestStandup = "daily_standup"
	DigestSummary = "daily_summary"
	DigestThemes  = "weekly_themes"
)

/*
Limits of digests: how many due destinations one pass reads, how many
themes a digest names and how many subjects a theme finder reads.
*/
const (
	digestBatch   = 50
	themesShown   = 5
	subjectsShown = 200
)

/*
Theme is one thing customers wrote about, with how many conversations.
*/
type Theme struct {
	Name  string
	Count int
}

/*
ThemeFinder groups a week's conversation subjects into themes.
*/
type ThemeFinder interface {
	/*
		Themes returns at most five themes of the subjects, the largest
		first.
	*/
	Themes(ctx context.Context, workspaceID ids.UUID, subjects []string) ([]Theme, error)
}

/*
Digests sends the digests that are due. Themes is nil when no theme
finder is set.
*/
type Digests struct {
	DB      *database.DB
	Clock   clock.Clock
	Channel *SlackChannel
	Links   Links
	Themes  ThemeFinder
}

/*
Due sends every digest that is due.
*/
func (d *Digests) Due(ctx context.Context) error {
	var errs []error

	seen := map[ids.UUID]bool{}

	for {
		due, err := models.New(d.DB).ListNotificationDigestsDue(ctx, models.ListNotificationDigestsDueParams{
			DueBefore: d.Clock.Now(), MaxRows: digestBatch,
		})
		if err != nil {
			return fmt.Errorf("list notification digests due: %w", err)
		}

		repeated := false

		for _, row := range due {
			repeated = repeated || seen[row.NotificationDestinationID]
			seen[row.NotificationDestinationID] = true
			errs = append(errs, d.Send(ctx, row.WorkspaceID, row.NotificationDestinationID))
		}

		if len(due) < digestBatch || repeated {
			return errors.Join(errs...)
		}
	}
}

/*
digest is what one destination's digests read, gathered in one
transaction.
*/
type digest struct {
	row       models.NotificationDestination
	workspace models.Workspace
	monday    bool
	standup   []models.DigestStandupRow
	nobody    int64
	summary   models.DigestSummaryRow
	oldest    *models.DigestOldestUnansweredRow
	tags      []models.DigestTopTagsRow
	subjects  []string
}

/*
Send sends one destination's digests if they are due, and schedules the
next ones.
*/
func (d *Digests) Send(ctx context.Context, ws, id ids.UUID) error {
	now := d.Clock.Now()

	var (
		in  digest
		due bool
	)

	err := d.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error

		q := models.New(tx)
		in, due, err = d.gather(ctx, q, dueDigest{workspace: ws, destination: id, now: now})
		if err != nil || !due {
			return err
		}

		return q.ScheduleNotificationDigest(ctx, models.ScheduleNotificationDigestParams{
			NextDigestAt: nextDigest(now, in.workspace.Timezone, hearingOf(in.row)), Now: now, WorkspaceID: ws, ID: id,
		})
	})
	if err != nil || !due {
		return err
	}

	var errs []error

	for _, card := range d.cards(ctx, in) {
		_, err := d.Channel.Post(ctx, *RouteOf(in.row), card)
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

/*
dueDigest names a destination whose digest may be due, and the time.
*/
type dueDigest struct {
	workspace   ids.UUID
	destination ids.UUID
	now         time.Time
}

/*
gather reads what the destination's digests show, reporting false when
none is due.
*/
func (d *Digests) gather(ctx context.Context, q *models.Queries, due dueDigest) (digest, bool, error) {
	ws, id, now := due.workspace, due.destination, due.now

	row, err := q.FindNotificationDestination(ctx, models.FindNotificationDestinationParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return digest{}, false, nil
	}

	if err != nil {
		return digest{}, false, fmt.Errorf("find notification destination: %w", err)
	}

	if row.Status != StatusActive || row.NextDigestAt == nil || row.NextDigestAt.After(now) {
		return digest{}, false, nil
	}

	w, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return digest{}, false, fmt.Errorf("find workspace: %w", err)
	}

	in := digest{row: row, workspace: w, monday: localDay(now, w.Timezone) == time.Monday}
	err = d.read(ctx, q, &in, now)

	return in, true, err
}

/*
read loads the numbers of every digest the destination gets.
*/
func (d *Digests) read(ctx context.Context, q *models.Queries, in *digest, now time.Time) error {
	ws := in.row.WorkspaceID

	var err error

	for _, kind := range in.row.Digests {
		switch kind {
		case DigestStandup:
			in.standup, in.nobody, err = readStandup(ctx, q, ws)
		case DigestSummary:
			in.summary, in.oldest, err = readSummary(ctx, q, ws, now)
		case DigestThemes:
			if in.monday {
				in.tags, in.subjects, err = readThemes(ctx, q, ws, now)
			}
		}

		if err != nil {
			return fmt.Errorf("read %s digest: %w", kind, err)
		}
	}

	return nil
}

/*
readStandup loads each teammate's open queue and what nobody holds.
*/
func readStandup(ctx context.Context, q *models.Queries, ws ids.UUID) ([]models.DigestStandupRow, int64, error) {
	rows, err := q.DigestStandup(ctx, ws)
	if err != nil {
		return nil, 0, err
	}

	nobody, err := q.DigestUnassigned(ctx, ws)

	return rows, nobody, err
}

/*
readSummary loads the last day's numbers and the conversation waiting
longest without a reply.
*/
func readSummary(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time,
) (models.DigestSummaryRow, *models.DigestOldestUnansweredRow, error) {
	summary, err := q.DigestSummary(ctx, models.DigestSummaryParams{
		Since: now.Add(-24 * time.Hour), Until: now, WorkspaceID: ws,
	})
	if err != nil {
		return summary, nil, err
	}

	oldest, err := q.DigestOldestUnanswered(ctx, ws)
	if errors.Is(err, database.ErrNotFound) {
		return summary, nil, nil
	}

	return summary, &oldest, err
}

/*
readThemes loads last week's most used tags and its subjects.
*/
func readThemes(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time,
) ([]models.DigestTopTagsRow, []string, error) {
	since := now.AddDate(0, 0, -7)

	tags, err := q.DigestTopTags(ctx, models.DigestTopTagsParams{
		WorkspaceID: ws, Since: since, Until: now, RowLimit: themesShown,
	})
	if err != nil {
		return nil, nil, err
	}

	subjects, err := q.DigestSubjects(ctx, models.DigestSubjectsParams{
		WorkspaceID: ws, Since: since, Until: now, RowLimit: subjectsShown,
	})

	return tags, subjects, err
}

/*
hearingOf is a destination's digest setup.
*/
func hearingOf(row models.NotificationDestination) Hearing {
	return Hearing{Digests: row.Digests, DigestTime: row.DigestTime}
}

/*
localDay is the weekday in the zone.
*/
func localDay(now time.Time, zone string) time.Weekday {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}

	return now.In(loc).Weekday()
}
