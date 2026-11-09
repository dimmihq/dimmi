/*
|--------------------------------------------------------------------------
| Listing Entries and Labels
|--------------------------------------------------------------------------
|
| The team's list of a changelog's entries, a page at a time, narrowed
| by state, label and title, with how many entries are in each state.
| Labels are a changelog's kinds of change, such as New, Improved and
| Fixed, which a changelog starts with. A help center holds at most
| twenty, and deleting one keeps its entries.
|
*/

package changelog

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing labels.
*/
var (
	ErrLabelNotFound = fault.New(fault.NotFound, "changelog_label_not_found",
		"No changelog label of this workspace has this id.")
	ErrLabelInvalid = fault.New(fault.Invalid, "label_invalid",
		"Labels have a name of 1 to 40 characters and a color like #22c55e.").OnField("name")
	ErrLabelTaken = fault.New(fault.Conflict, "label_taken",
		"Another label already has this name.").OnField("name")
	ErrTooManyLabels = fault.New(fault.Invalid, "too_many_labels",
		"A changelog has at most 20 labels.")
)

/*
TotalCap is the most a count of entries goes up to.
*/
const TotalCap = 10_000

/*
maxLabels is how many labels a changelog holds at most.
*/
const maxLabels = 20

/*
defaultLabels returns the labels a changelog starts with.
*/
func defaultLabels() []struct{ name, color string } {
	return []struct{ name, color string }{{"New", "#22c55e"}, {"Improved", "#3b82f6"}, {"Fixed", "#f97316"}}
}

/*
EntryFilter narrows a page of a changelog's entries.
*/
type EntryFilter struct {
	HelpCenterID ids.UUID
	Status       string
	LabelID      *ids.UUID
	Query        string
	Limit        int
	Offset       int
}

/*
EntryPage is a page of entries, how many match in all up to TotalCap,
and how many of the changelog's entries are in each state.
*/
type EntryPage struct {
	Entries  []Entry
	Total    int64
	ByStatus map[string]int32
}

/*
List returns a page of a changelog's entries.
*/
func (e *Entries) List(ctx context.Context, m authz.Membership, f EntryFilter) (EntryPage, error) {
	if !m.Can("changelog:read") {
		return EntryPage{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	status, query := nonEmpty(f.Status), nonEmpty(strings.TrimSpace(f.Query))
	var out EntryPage
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		rows, err := q.ListChangelogEntries(ctx, models.ListChangelogEntriesParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, Status: status, LabelID: f.LabelID, Query: query,
			RowLimit: int32Of(f.Limit), RowOffset: int32Of(f.Offset),
		})
		if err != nil {
			return fmt.Errorf("list changelog entries: %w", err)
		}
		if out.Entries, err = dressAll(ctx, q, ws, rows); err != nil {
			return err
		}
		if out.Total, err = q.CountChangelogEntries(ctx, models.CountChangelogEntriesParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, Status: status, LabelID: f.LabelID, Query: query,
			Cap: TotalCap,
		}); err != nil {
			return fmt.Errorf("count changelog entries: %w", err)
		}
		out.ByStatus, err = countByStatus(ctx, q, ws, f.HelpCenterID)
		return err
	})
	return out, err
}

func countByStatus(ctx context.Context, q *models.Queries, ws, hc ids.UUID) (map[string]int32, error) {
	rows, err := q.CountChangelogEntriesByStatus(ctx, models.CountChangelogEntriesByStatusParams{
		WorkspaceID: ws, HelpCenterID: hc,
	})
	if err != nil {
		return nil, fmt.Errorf("count changelog entries by state: %w", err)
	}
	out := map[string]int32{"draft": 0, "scheduled": 0, "published": 0}
	for _, r := range rows {
		out[r.Status] = r.Entries
	}
	return out, nil
}
