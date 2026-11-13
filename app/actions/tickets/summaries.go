/*
|--------------------------------------------------------------------------
| Tracker Summaries
|--------------------------------------------------------------------------
|
| A tracker says at a glance what its customers report: a
| few sentences the cheap model writes from the first words of every
| linked conversation, how many reports there are, and which plans the
| reporters' companies are on. The written part is kept and written
| again only once new reports joined; the counts are always fresh.
| Without a model, or when it fails, the counts still show.
|
*/

package tickets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Summarizer writes what a tracker's customers report. A Filler that also
summarizes is used for it.
*/
type Summarizer interface {
	/*
		Summarize writes a few sentences on what the reports say.
	*/
	Summarize(ctx context.Context, q SummaryQuestion) (string, error)
}

/*
SummaryQuestion is a tracker's title and its customers' reports.
*/
type SummaryQuestion struct {
	WorkspaceID ids.UUID
	TicketID    ids.UUID
	Title       string
	Reports     []string
}

/*
PlanCount is how many reports came from companies on one plan; the
plan is empty for customers without one.
*/
type PlanCount struct {
	Plan  string
	Count int
}

/*
TrackerSummary is what a tracker's customers report.
*/
type TrackerSummary struct {
	Ticket      models.Ticket
	Summary     string
	Reports     int
	Plans       []PlanCount
	GeneratedAt *time.Time
}

/*
Summary returns what the tracker's customers report,
writing the summary again when new reports joined since.
*/
func (t *Tickets) Summary(ctx context.Context, m authz.Membership, ref Ref) (TrackerSummary, error) {
	if err := canRead(m); err != nil {
		return TrackerSummary{}, err
	}

	var (
		out  TrackerSummary
		kept *models.TicketSummary
		rows []models.ListTrackerReportsRow
	)

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		if found.Kind != KindTracker {
			return ErrNotTracker
		}

		out.Ticket = found

		read, err := readReports(ctx, q, found)
		rows, out.Reports, kept = read.rows, read.count, read.kept

		return err
	})
	if err != nil {
		return out, err
	}

	out.Plans = plansOf(rows)

	return t.withSummary(ctx, out, kept, rows)
}

/*
withSummary fills the written summary: the kept one while no report
joined since, else a new one when a model is there.
*/
func (t *Tickets) withSummary(
	ctx context.Context, out TrackerSummary, kept *models.TicketSummary, rows []models.ListTrackerReportsRow,
) (TrackerSummary, error) {
	if kept != nil {
		out.Summary, out.GeneratedAt = kept.Summary, &kept.GeneratedAt
	}

	writer, ok := t.Filler.(Summarizer)
	if !ok || len(rows) == 0 || (kept != nil && int(kept.ReportCount) == out.Reports) {
		return out, nil
	}

	reports := make([]string, len(rows))
	for i, r := range rows {
		reports[i] = clip(strings.TrimSpace(strOr(r.Subject)+"\n"+r.FirstMessage), 500)
	}

	text, err := writer.Summarize(ctx, SummaryQuestion{
		WorkspaceID: out.Ticket.WorkspaceID, TicketID: out.Ticket.ID, Title: out.Ticket.Title, Reports: reports,
	})
	if err != nil {
		return out, nil //nolint:nilerr // without a verdict the counts and the kept summary still show
	}

	now := t.Clock.Now()
	out.Summary, out.GeneratedAt = clip(strings.TrimSpace(text), 4000), &now

	err = t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SaveTicketSummary(ctx, models.SaveTicketSummaryParams{
			TicketID: out.Ticket.ID, WorkspaceID: out.Ticket.WorkspaceID, Summary: out.Summary,
			ReportCount: dbCount(out.Reports), Now: now,
		})
	})

	return out, err
}

/*
reports are a tracker's reports, how many conversations it reaches and
its kept summary, nil when none was written.
*/
type reports struct {
	rows  []models.ListTrackerReportsRow
	count int
	kept  *models.TicketSummary
}

/*
readReports reads the tracker's reports.
*/
func readReports(ctx context.Context, q *models.Queries, found models.Ticket) (reports, error) {
	rows, err := q.ListTrackerReports(ctx, models.ListTrackerReportsParams{
		WorkspaceID: found.WorkspaceID, TicketID: found.ID,
	})
	if err != nil {
		return reports{}, fmt.Errorf("list tracker reports: %w", err)
	}

	impact, err := q.CountTrackerImpact(ctx, models.CountTrackerImpactParams{
		TicketID: found.ID, WorkspaceID: found.WorkspaceID,
	})
	if err != nil {
		return reports{}, fmt.Errorf("count tracker reports: %w", err)
	}

	row, err := q.FindTicketSummary(ctx, models.FindTicketSummaryParams{
		WorkspaceID: found.WorkspaceID, TicketID: found.ID,
	})
	out := reports{rows: rows, count: int(impact.Conversations)}
	if err == nil {
		out.kept = &row
	}

	return out, nil
}

/*
plansOf counts the reports by their company's plan, most first.
*/
func plansOf(rows []models.ListTrackerReportsRow) []PlanCount {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.CompanyPlan]++
	}

	out := make([]PlanCount, 0, len(counts))
	for plan, n := range counts {
		out = append(out, PlanCount{Plan: plan, Count: n})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}

		return out[i].Plan < out[j].Plan
	})

	return out
}

/*
strOr reads optional text.
*/
func strOr(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
dbCount is a length as the database counts it.
*/
func dbCount(n int) int32 {
	return int32(min(n, 1<<30)) //nolint:gosec // capped above
}

/*
clip cuts text to at most n characters.
*/
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}

	return s
}
