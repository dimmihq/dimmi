/*
|--------------------------------------------------------------------------
| Waiting
|--------------------------------------------------------------------------
|
| A wait is worked out once, when the traveler arrives: a length of
| time, the next time of day on the chosen weekdays in the traveler's
| own zone, or a date the traveler carries, such as the end of their
| trial, moved by a few days. A date already past moves them on at once;
| a date they do not have takes the no-date path, or moves them on.
|
*/

package series

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
waitStep holds the traveler until the wait's time.
*/
func (r *run) waitStep(ctx context.Context, n Node) (outcome, error) {
	s := settingsOf[WaitSettings](n)

	switch s.Mode {
	case "time":
		zone, err := r.zone(ctx)
		if err != nil {
			return outcome{}, err
		}

		return outcome{wait: &wait{Kind: "timer", Until: nextClock(r.now, zone, s.At, s.Weekdays)}}, nil
	case "attribute":
		at, ok := r.dateOf(s.Attribute)
		if !ok {
			if r.g.hasPath(n.ID, PathNoDate) {
				return outcome{path: PathNoDate, result: "no_date"}, nil
			}

			return outcome{path: PathNext, result: "no_date"}, nil
		}

		until := at.AddDate(0, 0, s.OffsetDays)
		if !until.After(r.now) {
			return outcome{path: PathNext, result: "date_passed"}, nil
		}

		return outcome{wait: &wait{Kind: "timer", Until: until}}, nil
	}

	d, _ := durationOf(s.Amount, s.Unit)

	return outcome{wait: &wait{Kind: "timer", Until: r.now.Add(d)}}, nil
}

/*
engaged reports whether the traveler acted on what a message step of
this journey sent them.
*/
func (r *run) engaged(ctx context.Context, e Engagement) (bool, error) {
	got, err := r.q.SeriesNodeEngagement(ctx, models.SeriesNodeEngagementParams{
		WorkspaceID: r.e.WorkspaceID, SeriesEnrollmentID: &r.e.ID, NodeID: e.Node,
	})
	if err != nil {
		return false, wrap("read engagement", err)
	}

	switch e.Action {
	case "opened":
		return got.Opened, nil
	case "clicked":
		return got.Clicked, nil
	}

	return got.Replied, nil
}

/*
zone is the traveler's own time zone, else the workspace's.
*/
func (r *run) zone(ctx context.Context) (*time.Location, error) {
	if len(r.people) == 1 && r.people[0].Timezone != nil && r.e.Subject == SubjectContact {
		if loc, err := time.LoadLocation(*r.people[0].Timezone); err == nil {
			return loc, nil
		}
	}

	ws, err := r.q.FindWorkspace(ctx, r.e.WorkspaceID)
	if err != nil {
		return nil, wrap("find workspace", err)
	}

	if loc, err := time.LoadLocation(ws.Timezone); err == nil {
		return loc, nil
	}

	return time.UTC, nil
}

/*
dateOf reads a date attribute of the traveler: a standard one, or a
custom attribute written as a date, a timestamp or seconds since 1970.
*/
func (r *run) dateOf(key string) (time.Time, bool) {
	if len(r.people) == 0 {
		return time.Time{}, false
	}

	c := r.people[0]
	switch key {
	case "created_at":
		return c.CreatedAt, true
	case "last_seen_at":
		if c.LastSeenAt != nil {
			return *c.LastSeenAt, true
		}

		return time.Time{}, false
	}

	var attrs map[string]any
	_ = json.Unmarshal(c.CustomAttributes, &attrs)

	return parseDate(attrs[key])
}

/*
parseDate reads a date written as text or as seconds since 1970.
*/
func parseDate(v any) (time.Time, bool) {
	switch x := v.(type) {
	case float64:
		return time.Unix(int64(x), 0).UTC(), x > 0
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}

		if secs, err := strconv.ParseInt(x, 10, 64); err == nil && secs > 0 {
			return time.Unix(secs, 0).UTC(), true
		}
	}

	return time.Time{}, false
}

/*
nextClock returns the next time of day at, on one of the weekdays, in
the zone, after now.
*/
func nextClock(now time.Time, zone *time.Location, at string, weekdays []int) time.Time {
	h, m, _ := clockTime(at)
	local := now.In(zone)

	for i := range 8 {
		d := local.AddDate(0, 0, i)
		candidate := time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, zone)

		if candidate.After(now) && (len(weekdays) == 0 || slices.Contains(weekdays, int(candidate.Weekday()))) {
			return candidate
		}
	}

	return now.Add(day)
}
