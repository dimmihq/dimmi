/*
|--------------------------------------------------------------------------
| Sign-In Counts
|--------------------------------------------------------------------------
|
| How messenger sessions started each day, per workspace: signed in,
| anonymous, or refused and why. The counts let an owner see that their
| identity endpoint works before making sign-in required, and which
| mistake breaks it when it does not. They hold no per-person data.
| Counting is a map write on the session's path; each replica writes
| its counts once a minute and when it shuts down, so a crash loses at
| most a minute of what is only a gauge.
|
*/

package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The outcomes counted besides the code of a refused token.
*/
const (
	OutcomeVerified  = "verified"
	OutcomeAnonymous = "anonymous_started"
)

/*
StatsKept is how long daily counts are kept.
*/
const StatsKept = 90 * 24 * time.Hour

/*
statKey is one workspace's count of one outcome on one day.
*/
type statKey struct {
	workspaceID ids.UUID
	day         string
	outcome     string
}

/*
Stats counts session outcomes in memory and writes them down.
*/
type Stats struct {
	db     *database.DB
	clock  clock.Clock
	mu     sync.Mutex
	counts map[statKey]int64
}

/*
NewStats returns an empty counter.
*/
func NewStats(db *database.DB, clk clock.Clock) *Stats {
	return &Stats{db: db, clock: clk, counts: map[statKey]int64{}}
}

/*
Record counts one outcome. Only codes of identity failures and the two
outcomes above are counted, so the table never fills with unrelated
errors.
*/
func (s *Stats) Record(workspaceID ids.UUID, outcome string) {
	if s == nil || !counted(outcome) {
		return
	}
	key := statKey{workspaceID: workspaceID, day: s.clock.Now().UTC().Format(time.DateOnly), outcome: outcome}
	s.mu.Lock()
	s.counts[key]++
	s.mu.Unlock()
}

func counted(outcome string) bool {
	return outcome == OutcomeVerified || outcome == OutcomeAnonymous ||
		strings.HasPrefix(outcome, "token_") || outcome == "identity_not_configured"
}

/*
Flush adds what was counted to the table.
*/
func (s *Stats) Flush(ctx context.Context) error {
	s.mu.Lock()
	batch := s.counts
	s.counts = map[statKey]int64{}
	s.mu.Unlock()
	var errs []error
	for key, n := range batch {
		err := s.db.Transaction(tenancy.With(ctx, key.workspaceID.String()),
			func(ctx context.Context, tx *database.Tx) error {
				return models.New(tx).AddIdentityVerificationStat(ctx, models.AddIdentityVerificationStatParams{
					WorkspaceID: key.workspaceID, Day: key.day, Outcome: key.outcome, Count: n,
				})
			})
		if err != nil {
			errs = append(errs, fmt.Errorf("record sign-in counts: %w", err))
		}
	}
	return errors.Join(errs...)
}

/*
Start flushes on the interval until the returned function is called,
which stops and flushes once more.
*/
func (s *Stats) Start(ctx context.Context, interval time.Duration) func(context.Context) error {
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := s.clock.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.Chan():
				if err := s.Flush(ctx); err != nil {
					log.From(ctx).WarnContext(ctx, "flush sign-in counts", log.Err(err))
				}
			}
		}
	}()
	return func(ctx context.Context) error {
		close(done)
		<-stopped
		return s.Flush(ctx)
	}
}

/*
Prune drops every workspace's counts older than StatsKept.
*/
func (s *Stats) Prune(ctx context.Context) (int64, error) {
	before := s.clock.Now().UTC().Add(-StatsKept).Format(time.DateOnly)
	n, err := models.New(s.db).PruneIdentityVerificationStats(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("prune sign-in counts: %w", err)
	}
	return n, nil
}

/*
Day is one day's counts by outcome, the day written YYYY-MM-DD in UTC.
*/
type Day struct {
	Day      string
	Outcomes map[string]int64
}

/*
Summary is the counts of the last days: totals by outcome, each day's,
and the share of starts that were signed in.
*/
type Summary struct {
	Totals   map[string]int64
	Days     []Day
	Verified int64
	Invalid  int64
	Anon     int64
}

/*
Share is the part of session starts that were signed in, 0 to 1.
*/
func (s Summary) Share() float64 {
	total := s.Verified + s.Invalid + s.Anon
	if total == 0 {
		return 0
	}
	return float64(s.Verified) / float64(total)
}

/*
Starts is how many sessions started or were refused.
*/
func (s Summary) Starts() int64 {
	return s.Verified + s.Invalid + s.Anon
}

/*
Summarize returns the workspace's counts of the last days, 1 to 90.
*/
func (s *Stats) Summarize(ctx context.Context, m authz.Membership, days int) (Summary, error) {
	if err := RequireRead(m); err != nil {
		return Summary{}, err
	}
	var out Summary
	err := s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = summarize(ctx, models.New(tx), m.Workspace.ID, sinceDay(s.clock.Now(), days))
		return err
	})
	return out, err
}

/*
sinceDay is the first day of the last days, 1 to 90, counting today.
*/
func sinceDay(now time.Time, days int) string {
	days = min(max(days, 1), 90)
	return now.UTC().AddDate(0, 0, 1-days).Format(time.DateOnly)
}

/*
summarize reads the counts from the day on.
*/
func summarize(ctx context.Context, q *models.Queries, ws ids.UUID, since string) (Summary, error) {
	rows, err := q.ListIdentityVerificationStats(ctx, models.ListIdentityVerificationStatsParams{
		WorkspaceID: ws, Since: since,
	})
	if err != nil {
		return Summary{}, fmt.Errorf("list sign-in counts: %w", err)
	}
	out := Summary{Totals: map[string]int64{}}
	for _, r := range rows {
		out.Totals[r.Outcome] += r.Count
		if n := len(out.Days); n == 0 || out.Days[n-1].Day != r.Day {
			out.Days = append(out.Days, Day{Day: r.Day, Outcomes: map[string]int64{}})
		}
		out.Days[len(out.Days)-1].Outcomes[r.Outcome] += r.Count
		switch r.Outcome {
		case OutcomeVerified:
			out.Verified += r.Count
		case OutcomeAnonymous:
			out.Anon += r.Count
		default:
			out.Invalid += r.Count
		}
	}
	return out, nil
}
