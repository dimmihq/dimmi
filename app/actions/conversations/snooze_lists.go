/*
|--------------------------------------------------------------------------
| Snooze Lists
|--------------------------------------------------------------------------
|
| Admins set the snooze choices the workspace offers, and a team's own
| for the conversations it holds. Removing a list makes it inherit
| again. Anyone working conversations can read the lists and the choices
| that apply to one conversation, each with the moment it ends in their
| own time zone.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SnoozeLists are the lists set: the workspace's, nil when it uses the
built-in presets, and each team's.
*/
type SnoozeLists struct {
	Workspace []SnoozeOption
	Teams     map[ids.UUID][]SnoozeOption
}

/*
ListSnoozeLists returns the lists set.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListSnoozeLists struct {
	DB *database.DB
}

/*
Handle returns the lists.
*/
func (a *ListSnoozeLists) Handle(ctx context.Context, m authz.Membership) (SnoozeLists, error) {
	if err := CanRead(m); err != nil {
		return SnoozeLists{}, err
	}

	out := SnoozeLists{Teams: map[ids.UUID][]SnoozeOption{}}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListSnoozeLists(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list snooze lists: %w", err)
		}

		for _, row := range rows {
			var options []SnoozeOption
			if err := json.Unmarshal(row.Options, &options); err != nil {
				return fmt.Errorf("read snooze list: %w", err)
			}

			if row.TeamID == nil {
				out.Workspace = options
			} else {
				out.Teams[*row.TeamID] = options
			}
		}

		return nil
	})

	return out, err
}

/*
UpdateSnoozeListInput is who sets which list: the team's, or the
workspace's when Team is nil. Options nil removes the list.
*/
type UpdateSnoozeListInput struct {
	Membership authz.Membership
	Team       *ids.UUID
	Options    []SnoozeOption
}

/*
UpdateSnoozeList sets or removes a snooze list.

Rules:
  - The caller needs settings:write and inboxes:manage.
  - The options are valid; an empty list is refused, remove it instead.
  - The team belongs to the workspace.

Events: [appevents.SnoozeListUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidSnoozeOptions],
[exceptions.ErrNotFound], or a wrapped database error.
*/
type UpdateSnoozeList struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle sets or removes the list.
*/
func (a *UpdateSnoozeList) Handle(ctx context.Context, in UpdateSnoozeListInput) error {
	m := in.Membership
	if !m.Can("settings:write") || !m.Can("inboxes:manage") {
		return exceptions.ErrForbidden
	}

	if in.Options != nil {
		if err := ValidateSnoozeOptions(in.Options); err != nil {
			return err
		}
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if err := teamOf(ctx, q, m.Workspace.ID, in.Team); err != nil {
			return err
		}

		if err := a.save(ctx, q, in); err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.SnoozeListUpdated{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: a.Clock.Now(),
				Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
			},
			TeamID: in.Team,
		})
	})
}

/*
save writes or removes the list.
*/
func (a *UpdateSnoozeList) save(ctx context.Context, q *models.Queries, in UpdateSnoozeListInput) error {
	ws := in.Membership.Workspace.ID

	if in.Options == nil {
		if err := q.DeleteSnoozeList(ctx, models.DeleteSnoozeListParams{WorkspaceID: ws, TeamID: in.Team}); err != nil {
			return fmt.Errorf("remove snooze list: %w", err)
		}

		return nil
	}

	raw, err := json.Marshal(in.Options)
	if err != nil {
		return fmt.Errorf("encode snooze list: %w", err)
	}

	_, err = q.SaveSnoozeList(ctx, models.SaveSnoozeListParams{
		WorkspaceID: ws, TeamID: in.Team, Options: raw, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("save snooze list: %w", err)
	}

	return nil
}

/*
teamOf refuses a team the workspace does not have.
*/
func teamOf(ctx context.Context, q *models.Queries, workspace ids.UUID, team *ids.UUID) error {
	if team == nil {
		return nil
	}

	live, err := q.LiveViewTargets(ctx, models.LiveViewTargetsParams{WorkspaceID: workspace, Ids: []ids.UUID{*team}})
	if err != nil {
		return fmt.Errorf("find team: %w", err)
	}

	if len(live) == 0 {
		return exceptions.ErrNotFound
	}

	return nil
}

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
