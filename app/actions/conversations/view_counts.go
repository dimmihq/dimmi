/*
|--------------------------------------------------------------------------
| Saved View Counts
|--------------------------------------------------------------------------
|
| The sidebar counts the open conversations of the saved views a member
| pinned, at most twenty of them. A view's conditions can be expensive,
| so each count is kept for thirty seconds; a sidebar a little behind is
| fine, a slow one is not. A pinned view that is gone, or no longer
| compiles, is left out.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on the counts of saved views.
*/
const (
	maxPinnedCounts = 20
	viewCountTTL    = 30 * time.Second
)

/*
pinnedViews returns the saved views the member pinned in their sidebar.
*/
func pinnedViews(ctx context.Context, q *models.Queries, registry *ids.Registry, m authz.Membership) []ids.UUID {
	prefs, err := preferenceStore{q: q, ids: registry}.load(ctx, m)
	if err != nil {
		return nil
	}

	var sidebar struct {
		PinnedViews []string `json:"pinned_views"`
	}

	if json.Unmarshal(prefs.Sidebar, &sidebar) != nil {
		return nil
	}

	var out []ids.UUID

	for _, named := range sidebar.PinnedViews {
		id, err := registry.Decode(string(models.PrefixConversationView), named)
		if err == nil && len(out) < maxPinnedCounts {
			out = append(out, id)
		}
	}

	return out
}

/*
countViews counts the open conversations of each pinned view, from the
cache when a count is fresh.
*/
func (a *CountConversations) countViews(ctx context.Context, tx *database.Tx, m authz.Membership) map[ids.UUID]int64 {
	q := models.New(tx)
	out := map[ids.UUID]int64{}

	for _, id := range pinnedViews(ctx, q, a.IDs, m) {
		key := "view-count:" + m.Member.ID.String() + ":" + id.String()

		raw, err := a.Cache.Remember(ctx, key, viewCountTTL, func(ctx context.Context) ([]byte, error) {
			n, err := a.countView(ctx, tx, m, id)

			return []byte(strconv.FormatInt(n, 10)), err
		})
		if err != nil {
			continue
		}

		if n, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
			out[id] = n
		}
	}

	return out
}

/*
countView counts the open conversations of one saved view, up to the
cap.
*/
func (a *CountConversations) countView(
	ctx context.Context, tx *database.Tx, m authz.Membership, id ids.UUID,
) (int64, error) {
	q := models.New(tx)
	lister := &ListConversations{IDs: a.IDs, Clock: a.Clock}
	in := ListConversationsInput{Membership: m, Filters: Filters{View: ViewCustom, ViewID: id}}

	view, err := lister.applyCustomView(ctx, q, &in)
	if err != nil {
		return 0, err
	}

	list, _ := listOf(m, in.Filters, in.Sort)
	view.apply(&list)

	sql, args := list.Count(CountCap + 1)

	var n int64
	err = tx.QueryRow(ctx, sql, args...).Scan(&n)

	return n, err
}
