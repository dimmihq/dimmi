/*
|--------------------------------------------------------------------------
| Conversation Sorts
|--------------------------------------------------------------------------
|
| Each order of a list is a fixed row of keys ending in the id, so two
| conversations never tie and a page always starts exactly after the last
| row of the page before. Pages follow by the row's key values, never by
| offset, so a conversation arriving while someone pages neither repeats
| a row nor hides one. Nulls follow PostgreSQL's own default, which is
| what the indexes store: first when descending, last when ascending.
|
*/

package querybuilder

import (
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Sort is one order a list can have, named as the API names it.
*/
type Sort string

/*
The orders of a list.
*/
const (
	SortLatest          Sort = "-last_message_at"
	SortOldest          Sort = "last_message_at"
	SortWaiting         Sort = "waiting_since"
	SortNewestStarted   Sort = "-created_at"
	SortOldestStarted   Sort = "created_at"
	SortPriority        Sort = "-priority"
	SortPriorityWaiting Sort = "-priority,waiting_since"
	SortSnoozeEnds      Sort = "snoozed_until"
)

/*
Sorts returns every order a list can have.
*/
func Sorts() []Sort {
	return []Sort{
		SortLatest, SortOldest, SortWaiting, SortNewestStarted, SortOldestStarted, SortPriority, SortPriorityWaiting,
		SortSnoozeEnds,
	}
}

/*
Columns a sort walks.
*/
const (
	columnPriority  = "c.priority"
	columnActivity  = "c.last_message_at"
	columnWaiting   = "c.waiting_since"
	columnCreated   = "c.created_at"
	columnSnoozeEnd = "c.snoozed_until"
	columnID        = "c.id"
)

/*
sortKey is one column of an order. Nullable columns need care past a
null in a cursor.
*/
type sortKey struct {
	column   string
	desc     bool
	nullable bool
}

/*
keys returns the order's columns, the latest activity when the sort is
empty.
*/
func (s Sort) keys() []sortKey {
	switch s {
	case SortOldest:
		return []sortKey{{columnActivity, false, true}, {columnID, false, false}}
	case SortWaiting:
		return []sortKey{{columnWaiting, false, true}, {columnID, false, false}}
	case SortNewestStarted:
		return []sortKey{{columnCreated, true, false}, {columnID, true, false}}
	case SortOldestStarted:
		return []sortKey{{columnCreated, false, false}, {columnID, false, false}}
	case SortPriority:
		return []sortKey{{columnPriority, true, false}, {columnActivity, true, true}, {columnID, true, false}}
	case SortPriorityWaiting:
		return []sortKey{{columnPriority, true, false}, {columnWaiting, false, true}, {columnID, false, false}}
	case SortSnoozeEnds:
		return []sortKey{{columnSnoozeEnd, false, true}, {columnID, false, false}}
	case SortLatest:
	}

	return []sortKey{{columnActivity, true, true}, {columnID, true, false}}
}

/*
Keyset is a row's position in an order: the priority when the order
reads it, its time, nil when the row has none, and its id.
*/
type Keyset struct {
	Priority int16      `json:"p,omitempty"`
	At       *time.Time `json:"t,omitempty"`
	ID       ids.UUID   `json:"i"`
}

/*
TimeOf picks the time the order reads from the row's times.
*/
func (s Sort) TimeOf(lastMessage, waiting, created, snoozeEnds *time.Time) *time.Time {
	switch s {
	case SortWaiting, SortPriorityWaiting:
		return waiting
	case SortNewestStarted, SortOldestStarted:
		return created
	case SortSnoozeEnds:
		return snoozeEnds
	case SortLatest, SortOldest, SortPriority:
		return lastMessage
	}

	return lastMessage
}

/*
reversed returns the order walked the other way, which also mirrors
where nulls fall.
*/
func reversed(keys []sortKey) []sortKey {
	out := slices.Clone(keys)
	for i := range out {
		out[i].desc = !out[i].desc
	}

	return out
}

/*
orderBy writes the ORDER BY of the keys.
*/
func orderBy(keys []sortKey) string {
	parts := make([]string, len(keys))

	for i, key := range keys {
		parts[i] = key.column
		if key.desc {
			parts[i] += " DESC"
		}
	}

	return strings.Join(parts, ", ")
}

/*
keyset writes the predicate selecting the rows after the position. An
order with one direction and no null in the way compares as one row,
which the index serves as a range; any other spells each step out.
*/
func (b *builder) keyset(keys []sortKey, after Keyset) string {
	if sql, ok := b.rowCompare(keys, after); ok {
		return sql
	}

	return b.steps(keys, after)
}

/*
rowCompare writes the row comparison of the keys, and reports false,
having added nothing, when it cannot express them.
*/
func (b *builder) rowCompare(keys []sortKey, after Keyset) (string, bool) {
	for _, key := range keys {
		if key.desc != keys[0].desc || after.value(key.column) == nil {
			return "", false
		}
	}

	var columns, values, nullable []string

	for _, key := range keys {
		value := after.value(key.column)

		if key.nullable {
			nullable = append(nullable, key.column)
		}

		columns = append(columns, key.column)
		values = append(values, b.add(value))
	}

	op := " > "
	if keys[0].desc {
		op = " < "
	}

	sql := "(" + strings.Join(columns, ", ") + ")" + op + "(" + strings.Join(values, ", ") + ")"
	if keys[0].desc || len(nullable) == 0 {
		return sql, true
	}

	for _, column := range nullable {
		sql += " OR " + column + " IS NULL"
	}

	return sql, true
}

/*
steps writes the comparison key by key: past this key, or level with it
and past the rest. A null sorts first when descending and last when
ascending.
*/
func (b *builder) steps(keys []sortKey, after Keyset) string {
	key := keys[0]
	value := after.value(key.column)

	if len(keys) == 1 {
		return key.column + key.further() + b.add(value)
	}

	rest := b.steps(keys[1:], after)

	if value == nil {
		if key.desc {
			return key.column + " IS NOT NULL OR (" + key.column + " IS NULL AND (" + rest + "))"
		}

		return key.column + " IS NULL AND (" + rest + ")"
	}

	placeholder := b.add(value)
	beyond := key.column + key.further() + placeholder

	if key.nullable && !key.desc {
		beyond += " OR " + key.column + " IS NULL"
	}

	return beyond + " OR (" + key.column + " = " + placeholder + " AND (" + rest + "))"
}

/*
further returns the operator that moves along the key's order.
*/
func (k sortKey) further() string {
	if k.desc {
		return " < "
	}

	return " > "
}

/*
value returns the position's value for the column, nil when it has none.
*/
func (k Keyset) value(column string) any {
	switch column {
	case columnPriority:
		return k.Priority
	case columnID:
		return k.ID
	}

	if k.At == nil {
		return nil
	}

	return *k.At
}
