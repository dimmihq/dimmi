/*
|--------------------------------------------------------------------------
| Conversations By AI Attribute
|--------------------------------------------------------------------------
|
| One row per value of a choice or yes-or-no AI conversation attribute,
| for the conversations started in the range: how many there were, how
| many of those the AI answered in it resolved, and what customers
| rated them. Every option shows, found or not, and a last row holds the
| conversations the attribute was not found on yet. The attribute is
| the one the query names, else the first active one.
|
*/

package reports

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
notFound is the key and label of the row of conversations the attribute
was not found on.
*/
const (
	notFoundKey   = "none"
	notFoundLabel = "Not found yet"
)

/*
attributeColumns are the figures each value's row shows.
*/
func attributeColumns() []Column {
	return []Column{
		{Key: "conversations", Label: "Conversations", Unit: UnitCount},
		{Key: "resolution", Label: "AI resolved", Unit: UnitPercent},
		{Key: "satisfaction", Label: "CSAT", Unit: UnitPercent},
	}
}

/*
breakdownAttribute returns the attribute to split by: the one named,
refused when it is unknown or not a choice or yes or no, else the first
active one that can be; false when there is none.
*/
func (r reader) breakdownAttribute(ctx context.Context) (models.AiConversationAttribute, bool, error) {
	rows, err := models.New(r.tx).ListAIConversationAttributes(ctx, r.q.Membership.Workspace.ID)
	if err != nil {
		return models.AiConversationAttribute{}, false, fmt.Errorf("list ai attributes: %w", err)
	}

	named := r.q.Filters.AIBreakdown
	for _, a := range rows {
		splits := a.DataType == "choice" || a.DataType == "boolean"
		if named != "" && a.Key == named {
			if !splits {
				return a, false, exceptions.ErrNotFound.OnField("ai_breakdown")
			}

			return a, true, nil
		}

		if named == "" && splits && a.Status == "active" {
			return a, true, nil
		}
	}

	if named != "" {
		return models.AiConversationAttribute{}, false, exceptions.ErrNotFound.OnField("ai_breakdown")
	}

	return models.AiConversationAttribute{}, false, nil
}

/*
withAttributeTable adds the table of conversations by AI attribute to
the charts, when the workspace has an attribute to split by.
*/
func (r reader) withAttributeTable(ctx context.Context, charts []Chart) ([]Chart, error) {
	c, ok, err := r.attributeTable(ctx)
	if err != nil || !ok {
		return charts, err
	}

	return append(charts, c), nil
}

/*
attributeTable splits the range's conversations by an AI attribute's
value; false when the workspace has no attribute to split by.
*/
func (r reader) attributeTable(ctx context.Context) (Chart, bool, error) {
	a, ok, err := r.breakdownAttribute(ctx)
	if err != nil || !ok {
		return Chart{}, false, err
	}

	var s statement
	resolved := r.aiResolved(&s)
	group := "av.\"values\" ->> " + s.arg(a.Key) + "::text"

	rows, err := r.bound(s).read(ctx, figures{
		from:  fromFacts + " LEFT JOIN conversation_ai_values av ON av.conversation_id = f.conversation_id",
		dated: "f.started_at",
		cols: fmt.Sprintf("count(*)::float8, count(*) FILTER (WHERE f.ai_involved)::float8, "+
			"count(*) FILTER (WHERE %s)::float8, count(f.rating)::float8, "+
			"count(*) FILTER (WHERE f.rating >= 4)::float8", resolved),
	}, group, 5)
	if err != nil {
		return Chart{}, false, err
	}

	out := Chart{
		Key: "ai_attribute", Title: "Conversations by " + a.Label, Kind: ChartTable, Axis: AxisCategory,
		Drill: "new_conversations", Columns: attributeColumns(),
	}

	found := keyed(rows)
	for _, value := range attributeValues(a, rows) {
		out.Rows = append(out.Rows, attributeRow(a, value, found[value]))
	}

	slices.SortStableFunc(out.Rows, func(x, y Row) int {
		return cmp.Or(cmp.Compare(rank(x), rank(y)), cmp.Compare(first(y), first(x)))
	})

	return out, true, nil
}

/*
attributeValues returns every value a row shows: the attribute's own
values, then any other found, such as an option since removed, then the
conversations not found on when there are any.
*/
func attributeValues(a models.AiConversationAttribute, rows []values) []string {
	out := append([]string{}, a.Options...)
	if a.DataType == "boolean" {
		out = []string{"true", "false"}
	}

	none := false
	for _, v := range rows {
		switch {
		case v.key == "":
			none = true
		case !slices.Contains(out, v.key):
			out = append(out, v.key)
		}
	}

	if none {
		out = append(out, "")
	}

	return out
}

/*
attributeRow is one value's row, narrowing to its conversations. The
row of conversations the attribute was not found on narrows nothing.
*/
func attributeRow(a models.AiConversationAttribute, value string, v values) Row {
	vals := []*float64{num(v.count(0)), ratio(v.count(2), v.count(1)), ratio(v.count(4), v.count(3))}
	if value == "" {
		return Row{Key: notFoundKey, Label: notFoundLabel, Values: vals}
	}

	label := value
	if a.DataType == "boolean" {
		label = map[string]string{"true": "Yes", "false": "No"}[value]
	}

	return Row{Key: value, Label: cmp.Or(label, value), Values: vals, Filter: attributeFilter(a.Key, value)}
}

/*
attributeFilter is the filter that narrows a report to one value.
*/
func attributeFilter(key, value string) map[string]string {
	return map[string]string{"ai_attribute": key, "ai_attribute_value": value}
}

/*
rank sorts the row of conversations not found on after every value.
*/
func rank(r Row) int {
	if r.Key == notFoundKey {
		return 1
	}

	return 0
}
