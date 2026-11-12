/*
|--------------------------------------------------------------------------
| Building Charts
|--------------------------------------------------------------------------
|
| The helpers charts are built with: every day of the range on the axis,
| days without data left empty rather than skipped; groups named by
| their inbox, team or teammate, with the filter that narrows a report
| to them.
|
*/

package reports

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
days returns each local day of the range, "2006-01-02".
*/
func (q Query) days() []string {
	out := make([]string, 0, q.Days)
	for d := q.Start; d.Before(q.End); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format(time.DateOnly))
	}

	return out
}

/*
daily returns a series over every day of the range, read from grouped
values at the index; pick turns a row into the point's value.
*/
func daily(q Query, s Series, rows []values, pick func(values) *float64) Series {
	byDay := make(map[string]values, len(rows))
	for _, r := range rows {
		byDay[r.key] = r
	}

	for _, d := range q.days() {
		var y *float64
		if r, ok := byDay[d]; ok {
			y = pick(r)
		}

		s.Points = append(s.Points, Point{X: d, Y: y})
	}

	return s
}

/*
at picks the figure at the index.
*/
func at(i int) func(values) *float64 {
	return func(v values) *float64 { return v.at(i) }
}

/*
percentOf picks the percent one figure is of another.
*/
func percentOf(part, whole int) func(values) *float64 {
	return func(v values) *float64 { return ratio(v.count(part), v.count(whole)) }
}

/*
names are the workspace's inboxes, teams and teammates by id.
*/
type names map[string]map[ids.UUID]string

/*
loadNames reads the names groups show.
*/
func loadNames(ctx context.Context, tx *database.Tx, ws ids.UUID) (names, error) {
	rows, err := models.New(tx).ReportNames(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("read names: %w", err)
	}

	out := names{"inbox": {}, "team": {}, "member": {}}
	for _, r := range rows {
		out[r.Kind][r.ID] = r.Name
	}

	return out, nil
}

/*
groupKinds maps a group to the kind its names come from, the filter
that narrows to it and the public id prefix of its keys.
*/
func groupKinds() map[string][3]string {
	return map[string][3]string{
		groupInbox:  {"inbox", "inbox", string(models.PrefixInbox)},
		groupTeam:   {"team", "team", string(models.PrefixTeam)},
		groupMember: {"member", "member", string(models.PrefixMember)},
	}
}

/*
row names one group's row: the inbox, team or teammate it is, with
its filter; a channel or priority as it reads; no one as "None".
*/
func (n names) row(group, key string, vals []*float64) Row {
	r := Row{Key: key, Label: key, Values: vals}

	kind, ok := groupKinds()[group]
	if !ok {
		return plainRow(group, key, vals)
	}

	id, err := ids.Parse(key)
	if err != nil {
		r.Key, r.Label = "none", noneLabels()[kind[0]]

		return r
	}

	public := ids.Encode(kind[2], id)
	r.Key, r.Label, r.Filter = public, n[kind[0]][id], map[string]string{kind[1]: public}
	if r.Label == "" {
		r.Label = "Removed"
	}

	return r
}

/*
noneLabels names the row of conversations in no inbox, team or
teammate.
*/
func noneLabels() map[string]string {
	return map[string]string{"inbox": "No inbox", "team": "No team", "member": "Unassigned"}
}

/*
channelLabels names the channels as the dashboard does.
*/
func channelLabels() map[string]string {
	return map[string]string{
		"widget": "Messenger", "email": "Email", "slack": "Slack", "api": "API", "whatsapp": "WhatsApp",
		"sms": "SMS", "teams": "Microsoft Teams", "discord": "Discord", "telegram": "Telegram",
	}
}

/*
plainRow names a channel or priority row, keyed and filtered by the
name the filters take.
*/
func plainRow(group, key string, vals []*float64) Row {
	r := Row{Key: key, Label: key, Values: vals}

	if group == groupPriority {
		if level, err := strconv.Atoi(key); err == nil && level >= 0 && level < len(enums.ConversationPriorities()) {
			r.Key = string(enums.ConversationPriorities()[level])
			r.Label = strings.ToUpper(r.Key[:1]) + r.Key[1:]
		}
	} else if label, ok := channelLabels()[key]; ok {
		r.Label = label
	}

	r.Filter = map[string]string{filterKey(group): r.Key}

	return r
}

/*
filterKey returns the filter a plain group narrows by.
*/
func filterKey(group string) string {
	if group == groupPriority {
		return "priority"
	}

	return "channel"
}

/*
orNone sends no list as an empty one, which the queries read as all.
*/
func orNone[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}
