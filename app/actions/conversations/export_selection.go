/*
|--------------------------------------------------------------------------
| Export Selections
|--------------------------------------------------------------------------
|
| An export covers either the conversations a member picked or every
| conversation of a list: its filters, or a saved view. Either way only
| what the member may see is counted and written, judged each time the
| selection is read, so an export built later never shows what the
| member lost access to in the meantime.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
Selection names the conversations of an export: the ids picked, or the
filters of a list when Filters is set.
*/
type Selection struct {
	IDs     []ids.UUID `json:"ids,omitempty"`
	Filters *Filters   `json:"filters,omitempty"`
}

/*
Reading is one read of a selection: by whom, and how many rows at most.
*/
type Reading struct {
	Membership authz.Membership
	Selection  Selection
	Limit      int
}

/*
Selector reads a selection as the member sees it.
*/
type Selector struct {
	IDs   *ids.Registry
	Clock clock.Clock
}

/*
Count returns how many conversations the selection covers, up to the
reading's limit.
*/
func (s Selector) Count(ctx context.Context, tx *database.Tx, r Reading) (int, error) {
	l, ok, err := s.list(ctx, models.New(tx), r.Membership, r.Selection)
	if err != nil || !ok {
		return 0, err
	}

	sql, args := l.Count(r.Limit)

	var n int
	if err := tx.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count selection: %w", err)
	}

	return n, nil
}

/*
Select returns up to the reading's limit of conversations of the
selection, loaded, in the list's order.
*/
func (s Selector) Select(ctx context.Context, tx *database.Tx, r Reading) ([]Loaded, error) {
	q := models.New(tx)

	l, ok, err := s.list(ctx, q, r.Membership, r.Selection)
	if err != nil || !ok {
		return nil, err
	}

	l.Limit = r.Limit

	rows, err := selectConversations(ctx, tx, l)
	if err != nil {
		return nil, fmt.Errorf("select conversations: %w", err)
	}

	return Load(ctx, q, r.Membership.Member.ID, rows)
}

/*
list turns the selection into the list's query, reporting false for a
list that is empty whatever the data. Picked ids show in any status.
*/
func (s Selector) list(
	ctx context.Context, q *models.Queries, m authz.Membership, sel Selection,
) (querybuilder.Conversations, bool, error) {
	if sel.Filters == nil {
		return querybuilder.Conversations{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, Access: access(m),
			Extra: []querybuilder.Fragment{idsIn(sel.IDs)},
		}, true, nil
	}

	in := ListConversationsInput{Membership: m, Filters: *sel.Filters}
	if err := canList(m, in.Filters); err != nil {
		return querybuilder.Conversations{}, false, err
	}

	view, err := (&ListConversations{IDs: s.IDs, Clock: s.Clock}).applyCustomView(ctx, q, &in)
	if err != nil {
		return querybuilder.Conversations{}, false, err
	}

	l, ok := listOf(m, in.Filters, in.Sort)
	view.apply(&l)

	return l, ok, nil
}

/*
idsIn narrows a list to the conversations picked.
*/
func idsIn(picked []ids.UUID) querybuilder.Fragment {
	return func(next int) (string, []any) {
		return fmt.Sprintf("c.id = ANY($%d)", next), []any{picked}
	}
}
