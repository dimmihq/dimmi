/*
|--------------------------------------------------------------------------
| Running Clocks
|--------------------------------------------------------------------------
|
| A clock starts with its due time counted in the policy's hours and a
| warning some time before. Only one clock per target runs on a subject
| at a time; starting one that runs already changes nothing. A pause
| keeps the time left; resuming counts it again from now, so the due
| time moves by exactly the time spent waiting. A clock ends met when
| the work it measured happened, or canceled when it no longer applies.
|
*/

package sla

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
clocks works the clocks of one subject within a transaction.
*/
type clocks struct {
	q   *models.Queries
	now time.Time
	s   *subject
}

/*
begin starts every target the subject's state calls for: resolution,
and the first or next reply while the customer waits for the team.
Clocks of another policy end canceled. A subject that waits pauses at
once.
*/
func (c clocks) begin(ctx context.Context) error {
	if err := c.cancelOthers(ctx); err != nil {
		return err
	}

	targets := []string{TargetResolution}
	if conv := c.s.conv; conv != nil && conv.Status != "closed" {
		switch {
		case conv.FirstReplyAt == nil:
			targets = append(targets, TargetFirstReply)
		case conv.WaitingSince != nil:
			targets = append(targets, TargetNextReply)
		}
	}

	for _, t := range targets {
		if err := c.start(ctx, t); err != nil {
			return err
		}
	}

	return c.follow(ctx)
}

/*
follow pauses or resumes the subject's clocks to match whether it
waits now.
*/
func (c clocks) follow(ctx context.Context) error {
	if c.s.waiting() {
		return c.pause(ctx)
	}

	return c.resume(ctx)
}

/*
start starts the target's clock when the policy sets it.
*/
func (c clocks) start(ctx context.Context, target string) error {
	secs := c.s.seconds(target)
	if secs == nil {
		return nil
	}

	due := c.s.add(c.now, time.Duration(*secs)*time.Second)
	ticket, conv := c.s.keys()

	_, err := c.q.CreateSLAClock(ctx, models.CreateSLAClockParams{
		WorkspaceID: c.s.ws, PolicyID: c.s.policy.ID, TicketID: ticket, ConversationID: conv, Target: target,
		Now: c.now, DueAt: due, WarnAt: c.warnAt(c.now, due),
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("start sla clock: %w", err)
	}

	return nil
}

/*
warnAt is when a clock due then warns, nil without a warning or when
the warning would come before the clock started.
*/
func (c clocks) warnAt(from, due time.Time) *time.Time {
	if c.s.policy.WarnBeforeSeconds <= 0 {
		return nil
	}

	at := due.Add(-time.Duration(c.s.policy.WarnBeforeSeconds) * time.Second)
	if !at.After(from) {
		return nil
	}

	return &at
}

/*
open returns the subject's running and paused clocks.
*/
func (c clocks) open(ctx context.Context) ([]models.SlaClock, error) {
	ticket, conv := c.s.keys()
	list, err := c.q.ListOpenSubjectClocks(ctx, models.ListOpenSubjectClocksParams{
		WorkspaceID: c.s.ws, TicketID: ticket, ConversationID: conv,
	})
	if err != nil {
		return nil, fmt.Errorf("list open sla clocks: %w", err)
	}

	return list, nil
}

/*
end stops the open clocks of the targets in the state, met or canceled.
*/
func (c clocks) end(ctx context.Context, state string, targets ...string) error {
	list, err := c.open(ctx)
	if err != nil {
		return err
	}

	for _, clock := range list {
		if !slices.Contains(targets, clock.Target) {
			continue
		}

		clock.State, clock.StoppedAt, clock.PausedAt = state, &c.now, nil
		if err := c.save(ctx, clock); err != nil {
			return err
		}
	}

	return nil
}

/*
cancelOthers cancels the open clocks of policies other than the
subject's own.
*/
func (c clocks) cancelOthers(ctx context.Context) error {
	list, err := c.open(ctx)
	if err != nil {
		return err
	}

	for _, clock := range list {
		if clock.PolicyID == c.s.policy.ID {
			continue
		}

		clock.State, clock.StoppedAt, clock.PausedAt = StateCanceled, &c.now, nil
		if err := c.save(ctx, clock); err != nil {
			return err
		}
	}

	return nil
}

/*
pause holds every running clock, keeping its time left.
*/
func (c clocks) pause(ctx context.Context) error {
	list, err := c.open(ctx)
	if err != nil {
		return err
	}

	for _, clock := range list {
		if clock.State != StateRunning {
			continue
		}

		clock.State, clock.PausedAt = StatePaused, &c.now
		if err := c.save(ctx, clock); err != nil {
			return err
		}
	}

	return nil
}

/*
resume runs every paused clock again, its due time moved by the time
it waited.
*/
func (c clocks) resume(ctx context.Context) error {
	list, err := c.open(ctx)
	if err != nil {
		return err
	}

	for _, clock := range list {
		if clock.State != StatePaused || clock.PausedAt == nil {
			continue
		}

		left := c.s.between(*clock.PausedAt, clock.DueAt)
		clock.State, clock.PausedAt, clock.DueAt = StateRunning, nil, c.s.add(c.now, left)
		if clock.WarnedAt == nil {
			clock.WarnAt = c.warnAt(c.now, clock.DueAt)
		}

		if err := c.save(ctx, clock); err != nil {
			return err
		}
	}

	return nil
}

/*
save stores a clock's state and times.
*/
func (c clocks) save(ctx context.Context, clock models.SlaClock) error {
	_, err := c.q.SaveSLAClock(ctx, models.SaveSLAClockParams{
		State: clock.State, DueAt: clock.DueAt, WarnAt: clock.WarnAt, PausedAt: clock.PausedAt,
		WarnedAt: clock.WarnedAt, BreachedAt: clock.BreachedAt, StoppedAt: clock.StoppedAt, Now: c.now,
		WorkspaceID: clock.WorkspaceID, ID: clock.ID,
	})
	if err != nil {
		return fmt.Errorf("save sla clock: %w", err)
	}

	return nil
}
