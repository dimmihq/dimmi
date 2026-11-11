/*
|--------------------------------------------------------------------------
| Message Steps
|--------------------------------------------------------------------------
|
| A message step sends through its own broadcast to the traveler, or
| for an account to the people the step picks and to the company's
| Slack channels. Email waits for the day in the person's zone when the
| series keeps quiet hours, and nobody gets more series messages a day
| than the cap allows: a held message waits, it is never dropped. The
| journey moves on once every send of the visit went out or failed; a
| visit that reached nobody takes the could-not-send path, or ends the
| journey.
|
*/

package series

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
message sends a message step, or holds it for quiet hours or the cap.
*/
func (r *run) message(ctx context.Context, n Node, stepID string) (outcome, error) {
	if until, why, err := r.holdUntil(ctx, n); err != nil || !until.IsZero() {
		return outcome{wait: &wait{Kind: "cooloff", Until: until, Result: why}}, err
	}

	b, err := r.q.FindSeriesBroadcast(ctx, models.FindSeriesBroadcastParams{
		WorkspaceID: r.e.WorkspaceID, SeriesID: &r.e.SeriesID, SeriesNodeID: &n.ID,
	})
	if err != nil {
		return outcome{}, wrap("find step broadcast", err)
	}

	send := broadcasts.SeriesSend{Broadcast: b, EnrollmentID: r.e.ID}
	if send.StepID, err = parseID(stepID); err != nil {
		return outcome{}, fmt.Errorf("read step id: %w", err)
	}

	if err := r.recipients(ctx, n, &send); err != nil {
		return outcome{}, err
	}

	queued, err := r.s.Broadcasts.SendForSeries(ctx, r.tx, send)
	if err != nil {
		return outcome{}, err
	}

	if queued.Queued == 0 {
		return r.undelivered(n, strings.Join(queued.Reasons, ",")), nil
	}

	return outcome{wait: &wait{Kind: "send"}}, nil
}

/*
delivered moves on once every send of the visit settled.
*/
func (r *run) delivered(ctx context.Context, n Node, w wait) (outcome, error) {
	id, err := parseID(w.StepID)
	if err != nil {
		return outcome{}, fmt.Errorf("read step id: %w", err)
	}

	got, err := r.q.SeriesStepDeliveries(ctx, models.SeriesStepDeliveriesParams{
		WorkspaceID: r.e.WorkspaceID, SeriesStepID: &id,
	})
	if err != nil {
		return outcome{}, wrap("count deliveries", err)
	}

	switch {
	case got.Queued > 0:
		return outcome{stay: true}, nil
	case got.Sent > 0:
		return outcome{path: PathNext, result: "sent", details: map[string]any{"sent": got.Sent}}, nil
	}

	return r.undelivered(n, got.Reasons), nil
}

/*
undelivered takes the could-not-send path when the step has one, and
otherwise ends the journey.
*/
func (r *run) undelivered(n Node, reasons string) outcome {
	details := map[string]any{"reasons": reasons}
	if r.g.hasPath(n.ID, PathFailed) {
		return outcome{path: PathFailed, result: "not_sent", details: details}
	}

	return outcome{end: "undeliverable", result: "not_sent", details: details}
}

/*
recipients picks who a visit reaches: Slack channels for a Slack step,
the traveler for a contact series, and the people the step's rules
pick in an account.
*/
func (r *run) recipients(ctx context.Context, n Node, send *broadcasts.SeriesSend) error {
	if n.Kind == KindSlack {
		if r.company == nil {
			return nil
		}

		rooms, err := r.q.ListCompanySeriesRooms(ctx, models.ListCompanySeriesRoomsParams{
			WorkspaceID: r.e.WorkspaceID, CompanyID: r.company,
		})
		send.Rooms = rooms

		return wrap("list company rooms", err)
	}

	s := settingsOf[MessageSettings](n)
	if r.e.Subject == SubjectContact || len(s.Recipients) == 0 {
		send.Contacts = r.people

		return nil
	}

	picked, err := r.s.Audiences.Matching(ctx, r.q, audiences.MatchInput{
		WorkspaceID: r.e.WorkspaceID, Rules: s.Recipients,
	}, r.people)
	if err != nil {
		return err
	}

	keep := map[string]bool{}
	for _, id := range picked {
		keep[id.String()] = true
	}

	for _, c := range r.people {
		if keep[c.ID.String()] {
			send.Contacts = append(send.Contacts, c)
		}
	}

	return nil
}

/*
holdUntil returns when a message may leave, zero when it may leave now,
and why it waits: quiet hours for email, or the daily cap.
*/
func (r *run) holdUntil(ctx context.Context, n Node) (time.Time, string, error) {
	if r.e.Test || r.e.Subject != SubjectContact {
		return time.Time{}, "", nil
	}

	if n.Kind == KindEmail && r.g.spec.Quiet {
		zone, err := r.zone(ctx)
		if err != nil {
			return time.Time{}, "", err
		}

		if open := r.nextDaytime(zone); !open.IsZero() {
			return open, "quiet_hours", nil
		}
	}

	return r.overCap(ctx)
}

/*
nextDaytime returns when quiet hours end in the zone, zero while it is
day.
*/
func (r *run) nextDaytime(zone *time.Location) time.Time {
	sh, sm, okStart := clockTime(r.s.Config.QuietStart)
	eh, em, okEnd := clockTime(r.s.Config.QuietEnd)

	if !okStart || !okEnd {
		return time.Time{}
	}

	local := r.now.In(zone)
	minutes := local.Hour()*60 + local.Minute()

	if minutes >= sh*60+sm && minutes < eh*60+em {
		return time.Time{}
	}

	return nextClock(r.now, zone, r.s.Config.QuietStart, nil)
}

/*
overCap spends one of the contact's series messages of the day, or says
when the next one is free.
*/
func (r *run) overCap(ctx context.Context) (time.Time, string, error) {
	quota := r.s.Config.MaxMessagesPerDay
	if quota <= 0 || r.s.Limiter == nil || r.e.ContactID == nil {
		return time.Time{}, "", nil
	}

	res, err := r.s.Limiter.Attempt(ctx, "series:contact:"+r.e.ContactID.String(), quota, day)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("take series message: %w", err)
	}

	if res.Allowed {
		return time.Time{}, "", nil
	}

	return r.now.Add(max(res.RetryAfter, time.Minute)), "daily_cap", nil
}
