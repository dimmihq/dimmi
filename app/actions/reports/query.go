/*
|--------------------------------------------------------------------------
| Report Queries
|--------------------------------------------------------------------------
|
| Every report is asked the same way: a range of whole days in the
| workspace's time zone, the period before it to compare with, and
| filters on inbox, team, teammate, channel, tag, priority and whether
| the AI took part. Durations are counted in calendar time, or in the
| time the inbox's office hours were open. Who asks decides what is
| counted: a teammate who may see only some conversations sees reports
| of only those.
|
*/

package reports

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The limits on a range, and the default one.
*/
const (
	MaxDays     = 400
	DefaultDays = 30
)

/*
The values of the AI filter.
*/
const (
	AIInvolved    = "involved"
	AINotInvolved = "not_involved"
)

/*
Filters narrow what a report counts. From and To are whole days,
"2006-01-02", To included; empty means the last thirty days.
*/
type Filters struct {
	From          string
	To            string
	Compare       bool
	Inboxes       []ids.UUID
	Teams         []ids.UUID
	Members       []ids.UUID
	Channels      []string
	Tags          []ids.UUID
	Priorities    []int16
	AI            string
	BusinessHours bool
}

/*
Query is one request for a report: who asks, what they filtered, the
range as moments, and the moment it is asked.
*/
type Query struct {
	Membership authz.Membership
	Filters    Filters
	Location   *time.Location
	Start      time.Time
	End        time.Time
	Days       int
	Now        time.Time
}

/*
NewQuery reads the filters' range in the workspace's time zone,
refusing days that cannot be read, an end before the start and a range
longer than MaxDays.
*/
func NewQuery(m authz.Membership, f Filters, now time.Time) (Query, error) {
	loc := time.UTC
	if l, err := time.LoadLocation(m.Workspace.Timezone); err == nil && m.Workspace.Timezone != "" {
		loc = l
	}

	today := midnight(now.In(loc))
	start, end := today.AddDate(0, 0, 1-DefaultDays), today.AddDate(0, 0, 1)

	if f.From != "" || f.To != "" {
		var err error
		if start, end, err = parseRange(f, today, loc); err != nil {
			return Query{}, err
		}
	}

	days := int(end.Sub(start).Hours()/24 + 0.5)
	if days > MaxDays {
		return Query{}, exceptions.ErrReportRange.OnField("from")
	}

	f.From, f.To = start.Format(time.DateOnly), end.AddDate(0, 0, -1).Format(time.DateOnly)

	return Query{Membership: m, Filters: f, Location: loc, Start: start, End: end, Days: days, Now: now}, nil
}

/*
parseRange reads the from and to days, a missing one defaulting to
today or thirty days before the end.
*/
func parseRange(f Filters, today time.Time, loc *time.Location) (time.Time, time.Time, error) {
	end := today
	if f.To != "" {
		t, err := time.ParseInLocation(time.DateOnly, f.To, loc)
		if err != nil {
			return t, t, exceptions.ErrReportRange.OnField("to")
		}

		end = t
	}

	start := end.AddDate(0, 0, 1-DefaultDays)
	if f.From != "" {
		t, err := time.ParseInLocation(time.DateOnly, f.From, loc)
		if err != nil {
			return t, t, exceptions.ErrReportRange.OnField("from")
		}

		start = t
	}

	if end.Before(start) {
		return start, end, exceptions.ErrReportRange.OnField("to")
	}

	return start, end.AddDate(0, 0, 1), nil
}

/*
Previous returns the query for the period of the same length just
before.
*/
func (q Query) Previous() Query {
	p := q
	p.End = q.Start
	p.Start = q.Start.AddDate(0, 0, -q.Days)
	p.Filters.Compare = false

	return p
}

/*
midnight returns the start of the moment's day in its location.
*/
func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
