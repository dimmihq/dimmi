/*
|--------------------------------------------------------------------------
| The Fifteen-Minute Re-check
|--------------------------------------------------------------------------
|
| A webhook can be missed, so every open link not checked in fifteen
| minutes is read again from its tool and applied the same way a
| delivery would be. Nothing is dropped in silence: an issue the tool no
| longer shows, such as one in a Linear team the app was removed from,
| keeps its link with a warning saying so, and a tool that fails keeps
| the error on the link until a later check works.
|
*/

package issues

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The re-check's pace: links older than syncEvery are read again, at
most syncBatch per run.
*/
const (
	syncEvery = 15 * time.Minute
	syncBatch = 200
)

/*
Sweep re-checks the links due.
*/
func (s *Issues) Sweep(ctx context.Context) error {
	var due []models.TicketIssuesDueSyncRow

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		due, err = models.New(tx).TicketIssuesDueSync(ctx, models.TicketIssuesDueSyncParams{
			SyncedBefore: s.Clock.Now().Add(-syncEvery), RowLimit: syncBatch,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list issues to sync: %w", err)
	}

	for _, d := range due {
		if err := s.Check(ctx, d.WorkspaceID, d.ID); err != nil {
			log.From(ctx).WarnContext(ctx, "sync linked issue", log.String("issue_id", d.ID.String()), log.Err(err))
		}
	}

	return nil
}

/*
Check reads one link's issue from its tool and applies it.
*/
func (s *Issues) Check(ctx context.Context, ws, id ids.UUID) error {
	var link models.TicketIssue

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		link, err = q.FindTicketIssue(ctx, models.FindTicketIssueParams{WorkspaceID: ws, ID: id})

		return err
	})
	if err != nil || link.InstallationID == nil {
		return err
	}

	tl, err := s.toolFor(ctx, ws, *link.InstallationID)
	if err != nil {
		return s.fail(ctx, link, "The connection to "+Label(link.Provider)+" is not working. Reconnect it.")
	}

	is, err := tl.tracker.Read(ctx, tl.call, first(link.ExternalID, link.Key))

	switch {
	case errors.Is(err, trackers.ErrNotFound), errors.Is(err, trackers.ErrHidden):
		return s.warn(ctx, link, hiddenWarning(link))
	case err != nil:
		return s.fail(ctx, link, clip(err.Error(), 2000))
	}

	s.watch(ctx, tl, is)

	return s.applyIssue(ctx, ws, id, is)
}

/*
hiddenWarning says why an issue can no longer be read.
*/
func hiddenWarning(link models.TicketIssue) string {
	if link.Provider == "linear" {
		team := first(link.TeamName, "its team")

		return clip("dimmi cannot see "+link.Key+" any more: the Linear app has no access to "+team+
			". Add the app to the team in Linear, or unlink the issue.", 500)
	}

	return clip(Label(link.Provider)+" no longer shows "+link.Key+" to dimmi. It may have been deleted or moved "+
		"where the connection cannot see it.", 500)
}

/*
warn keeps a warning on the link.
*/
func (s *Issues) warn(ctx context.Context, link models.TicketIssue, warning string) error {
	return s.inWorkspace(ctx, link.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		return q.WarnTicketIssue(ctx, models.WarnTicketIssueParams{
			Warning: &warning, Now: s.Clock.Now(), WorkspaceID: link.WorkspaceID, ID: link.ID,
		})
	})
}

/*
fail keeps the failure of the check on the link.
*/
func (s *Issues) fail(ctx context.Context, link models.TicketIssue, message string) error {
	return s.inWorkspace(ctx, link.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		now := s.Clock.Now()

		return q.FailTicketIssueSync(ctx, models.FailTicketIssueSyncParams{
			SyncError: &message, Now: &now, WorkspaceID: link.WorkspaceID, ID: link.ID,
		})
	})
}
