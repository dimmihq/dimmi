/*
|--------------------------------------------------------------------------
| Tracker Suggestions And Workspace Settings
|--------------------------------------------------------------------------
|
| The panel of a conversation asks which open trackers the AI thinks it
| reports, so a teammate adds it with one click: "Looks like CSV export
| crashes (54 others). Add to tracker?". The workspace's ticket
| settings say whether the best match is linked without asking.
|
*/

package tickets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceSettings are the workspace's ticket switches: whether a new
conversation is linked to the tracker it matches without asking.
*/
type WorkspaceSettings struct {
	AutoLinkDuplicates bool `json:"auto_link_duplicates"`
}

/*
readSettings reads the workspace's ticket settings.
*/
func readSettings(ctx context.Context, q *models.Queries, ws ids.UUID) (WorkspaceSettings, error) {
	raw, err := q.FindWorkspaceTicketSettings(ctx, ws)
	if err != nil {
		return WorkspaceSettings{}, fmt.Errorf("read ticket settings: %w", err)
	}

	var out WorkspaceSettings
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}

	return out, nil
}

/*
TrackerSuggestions returns the open trackers suggested for a
conversation the member may see, best first, leaving out those it
joined since.
*/
func (t *Tickets) TrackerSuggestions(
	ctx context.Context, m authz.Membership, ref conversations.Ref,
) ([]models.ListTrackerSuggestionsRow, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []models.ListTrackerSuggestionsRow

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		rows, err := q.ListTrackerSuggestions(ctx, models.ListTrackerSuggestionsParams{
			WorkspaceID: m.Workspace.ID, ConversationID: c.ID,
		})
		if err != nil {
			return fmt.Errorf("list tracker suggestions: %w", err)
		}

		for _, r := range rows {
			if r.Kind == KindTracker {
				out = append(out, r)
			}
		}

		return nil
	})

	return out, err
}
