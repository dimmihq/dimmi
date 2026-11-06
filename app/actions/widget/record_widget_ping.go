/*
|--------------------------------------------------------------------------
| Record Widget Ping
|--------------------------------------------------------------------------
|
| As a visitor moves through a site, their messenger says where they are,
| so the team sees the page a question came from. It says so at most once
| every ten seconds per session; anything faster changes nothing. Pages are
| cleaned before they are kept, the same as at session start.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
pingThrottle is how often a session's page is written down.
*/
const pingThrottle = 10 * time.Second

/*
RecordWidgetPing records where visitors are.
*/
type RecordWidgetPing struct {
	DB    *database.DB
	Clock clock.Clock
	Cache *cache.Store
}

/*
Handle records the page and context the visitor's browser reports.
*/
func (a *RecordWidgetPing) Handle(
	ctx context.Context, s models.WidgetSession, page *Page, change map[string]any,
) error {
	admitted, err := a.Cache.Add(ctx, "widget:ping:"+s.ID.String(), []byte{1}, pingThrottle)
	if err != nil {
		return fmt.Errorf("throttle widget ping: %w", err)
	}

	if !admitted {
		return nil
	}

	now := a.Clock.Now()
	current, recent := s.CurrentPage, s.RecentPages

	if page != nil {
		if cleaned, ok := cleanPage(*page, now); ok {
			var pages []Page
			_ = json.Unmarshal(s.RecentPages, &pages)

			current, _ = json.Marshal(cleaned)
			recent, _ = json.Marshal(pushPage(pages, cleaned))
		}
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).UpdateWidgetSessionPage(ctx, models.UpdateWidgetSessionPageParams{
			CurrentPage: current, RecentPages: recent, Context: cleanContext(s.Context, change), Now: now,
			WorkspaceID: s.WorkspaceID, ID: s.ID,
		})
		if err != nil {
			return fmt.Errorf("record widget ping: %w", err)
		}

		return nil
	})
}
