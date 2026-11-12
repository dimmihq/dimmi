//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo Service Levels
|--------------------------------------------------------------------------
|
| Two service level policies, found by name first, and the clocks of
| each demo ticket under its policy: the first reply mostly met, some
| breached, and resolution met or breached by when the ticket resolved,
| breached and still open when it waits past its target, or running
| while it still has time. Clocks are written at the moments they
| started and ended, as the clock sweep would have left them.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sla"
	"github.com/dimmihq/dimmi/app/models"
)

/*
seedPolicies adds the demo's service level policies unless they exist.
*/
func (d *ticketDemo) seedPolicies(ctx context.Context) error {
	if d.app.Actions.SLA == nil {
		return nil
	}

	have, err := d.app.Actions.SLA.List(ctx, d.owner)
	if err != nil {
		return err
	}

	specs := []struct {
		name                    string
		first, next, resolution int32
	}{{"Enterprise", 3600, 4 * 3600, 2 * 86400}, {"Standard", 4 * 3600, 8 * 3600, 5 * 86400}}

	for _, s := range specs {
		found := false
		for _, p := range have {
			if p.Name == s.name && p.ArchivedAt == nil {
				d.policies, found = append(d.policies, p.ID), true
			}
		}

		if found {
			continue
		}

		p, err := d.app.Actions.SLA.Create(ctx, d.owner, sla.Input{
			Name: &s.name, FirstReply: &s.first, NextReply: &s.next, Resolution: &s.resolution,
		})
		if err != nil {
			return err
		}

		d.policies = append(d.policies, p.ID)
	}

	return nil
}

/*
demoClock is one clock to write: its target, when it started and was
due, and when it was met or breached, nil while it runs.
*/
type demoClock struct {
	policy        ids.UUID
	target        string
	start, due    time.Time
	met, breached *time.Time
}

/*
clocks writes the ticket's first reply and resolution clocks.
*/
func (d *ticketDemo) clocks(ctx context.Context, run demoRun, t models.Ticket) error {
	if t.SlaPolicyID == nil {
		return nil
	}

	return d.app.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		p, err := q.FindSLAPolicy(ctx, models.FindSLAPolicyParams{WorkspaceID: t.WorkspaceID, ID: *t.SlaPolicyID})
		if err != nil {
			return fmt.Errorf("find policy: %w", err)
		}

		for _, c := range demoClocks(run, t, p) {
			if err := writeClock(ctx, q, t, c); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
demoClocks decides how the ticket's clocks went.
*/
func demoClocks(run demoRun, t models.Ticket, p models.SlaPolicy) []demoClock {
	i, opened, now := run.i, run.at, run.now

	var out []demoClock

	if p.FirstReplySeconds != nil {
		limit := time.Duration(*p.FirstReplySeconds) * time.Second
		c := demoClock{policy: p.ID, target: "first_reply", start: opened, due: opened.Add(limit)}

		switch {
		case roll(100, "first", i) < 80:
			c.met = new(opened.Add(limit * time.Duration(10+roll(80, "took", i)) / 100))
		case c.due.Before(now):
			c.breached = &c.due
		}

		if c.met == nil || c.met.Before(now) {
			out = append(out, c)
		}
	}

	if p.ResolutionSeconds != nil {
		limit := time.Duration(*p.ResolutionSeconds) * time.Second
		c := demoClock{policy: p.ID, target: "resolution", start: opened, due: opened.Add(limit)}

		switch {
		case t.ResolvedAt != nil && !t.ResolvedAt.After(c.due):
			c.met = t.ResolvedAt
		case c.due.Before(now):
			c.breached = &c.due
		}

		out = append(out, c)
	}

	return out
}

/*
writeClock starts the clock at its start and leaves it as it ended.
*/
func writeClock(ctx context.Context, q *models.Queries, t models.Ticket, c demoClock) error {
	row, err := q.CreateSLAClock(ctx, models.CreateSLAClockParams{
		WorkspaceID: t.WorkspaceID, PolicyID: c.policy, TicketID: &t.ID, Target: c.target, Now: c.start, DueAt: c.due,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start %s clock: %w", c.target, err)
	}

	p := models.SaveSLAClockParams{
		State: "running", DueAt: c.due, Now: c.start, WorkspaceID: t.WorkspaceID, ID: row.ID,
	}

	switch {
	case c.met != nil:
		p.State, p.StoppedAt, p.Now = "met", c.met, *c.met
	case c.breached != nil:
		p.State, p.BreachedAt, p.Now = "breached", c.breached, *c.breached
	default:
		return nil
	}

	if _, err := q.SaveSLAClock(ctx, p); err != nil {
		return fmt.Errorf("end %s clock: %w", c.target, err)
	}

	return nil
}
