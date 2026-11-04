/*
|--------------------------------------------------------------------------
| Snooze Choices
|--------------------------------------------------------------------------
|
| The snooze choices one conversation offers, from its team's list, the
| workspace's or the built-in presets, each with the moment it ends in
| the caller's own time zone, the workspace's when they set none.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ResolvedOption is a snooze option with the moment it ends for the
caller.
*/
type ResolvedOption struct {
	SnoozeOption

	Until time.Time
}

/*
SnoozeOptionsInput is who asks about which conversation.
*/
type SnoozeOptionsInput struct {
	Membership authz.Membership
	Ref        Ref
}

/*
ShowSnoozeOptions returns the snooze choices of a conversation, each
with the moment it ends in the caller's time zone.

Rules:
  - The caller needs conversations:read and sees the conversation.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowSnoozeOptions struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the options.
*/
func (a *ShowSnoozeOptions) Handle(ctx context.Context, in SnoozeOptionsInput) ([]ResolvedOption, error) {
	if err := CanRead(in.Membership); err != nil {
		return nil, err
	}

	var out []ResolvedOption

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := Find(ctx, q, in.Membership.Workspace.ID, in.Ref)
		if err != nil {
			return err
		}

		options, err := snoozeOptionsFor(ctx, q, c.WorkspaceID, c.AssigneeTeamID)
		if err != nil {
			return err
		}

		loc, err := callerLocation(ctx, q, in.Membership)
		if err != nil {
			return err
		}

		now := a.Clock.Now()
		for _, o := range options {
			out = append(out, ResolvedOption{SnoozeOption: o, Until: ResolveSnoozeOption(o, now, loc).UTC()})
		}

		return nil
	})

	return out, err
}

/*
callerLocation returns the caller's time zone, the workspace's when
they set none.
*/
func callerLocation(ctx context.Context, q *models.Queries, m authz.Membership) (*time.Location, error) {
	members, err := Members(ctx, q, m.Workspace.ID, []ids.UUID{m.Member.ID})
	if err != nil {
		return nil, fmt.Errorf("load the caller: %w", err)
	}

	member, ok := members[m.Member.ID]
	if !ok {
		return nil, errors.New("load the caller: not found")
	}

	return Location(member.User.Timezone, m.Workspace), nil
}
