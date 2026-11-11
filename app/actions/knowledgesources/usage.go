/*
|--------------------------------------------------------------------------
| Source Usage
|--------------------------------------------------------------------------
|
| What reading a source cost lands in the month's usage, beside what
| embedding it cost, so it can be shown on the source and billed on.
| A crawl through a hosted browser reports how long the browser ran,
| priced per browser hour. The month's usage is listed per source, a
| source that was removed keeping its line.
|
*/

package knowledgesources

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
browserHourMs is the milliseconds in a browser hour.
*/
const browserHourMs = 3_600_000

/*
recordCrawl adds the browser time a listing page used to the source's
month.
*/
func (e *Engine) recordCrawl(ctx context.Context, q *models.Queries, st runState, u sources.Usage) error {
	if u.BrowserMs <= 0 {
		return nil
	}

	now := e.Clock.Now()
	source := st.source.ID

	err := q.AddKnowledgeUsage(ctx, models.AddKnowledgeUsageParams{
		WorkspaceID: st.source.WorkspaceID, PeriodStart: database.Date(knowledge.MonthStart(now)),
		SourceID: &source, Kind: "crawl", ModelKey: u.Service, BrowserMs: u.BrowserMs, Requests: 1,
		CostMicros: (u.BrowserMs*e.BrowserHourMicros + browserHourMs/2) / browserHourMs,
		Unpriced:   e.BrowserHourMicros <= 0, Now: now,
	})
	if err != nil {
		return fmt.Errorf("add crawl usage: %w", err)
	}

	return nil
}

/*
UsageLine is one source's usage of a month, or the workspace's own
articles and conversations when SourceID is nil: tokens embedded, of
them estimated, browser time, calls, the cost in millionths of a
dollar, and whether part of it has no price.
*/
type UsageLine struct {
	SourceID        *ids.UUID
	SourceName      string
	SourceType      string
	Kind            string
	InputTokens     int64
	EstimatedTokens int64
	BrowserMs       int64
	Requests        int64
	CostMicros      int64
	Unpriced        bool
}

/*
Usage lists the month's knowledge usage of the workspace, the most
costly first.
*/
func (e *Engine) Usage(ctx context.Context, m authz.Membership) ([]UsageLine, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.ListKnowledgeUsageRow

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error

		rows, err = q.ListKnowledgeUsage(ctx, models.ListKnowledgeUsageParams{
			WorkspaceID: m.Workspace.ID, PeriodStart: database.Date(knowledge.MonthStart(e.Clock.Now())),
		})

		return wrap("list knowledge usage", err)
	})
	if err != nil {
		return nil, err
	}

	out := make([]UsageLine, 0, len(rows))
	for _, r := range rows {
		line := UsageLine{
			SourceName: r.SourceName, SourceType: r.SourceType, Kind: r.Kind, InputTokens: r.InputTokens,
			EstimatedTokens: r.EstimatedTokens, BrowserMs: r.BrowserMs, Requests: r.Requests,
			CostMicros: r.CostMicros, Unpriced: r.Unpriced,
		}
		line.SourceID = r.SourceID

		out = append(out, line)
	}

	return out, nil
}
