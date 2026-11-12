/*
|--------------------------------------------------------------------------
| Service Level Policies
|--------------------------------------------------------------------------
|
| Every member who reads conversations reads the policies; members who
| set tickets up change them. A policy needs at least one target, each
| between a minute and a year, and names only schedules, teams and
| ticket types of its workspace. Archiving keeps the policy for the
| clocks that ended under it and cancels the ones still running.
|
*/

package sla

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Input is a policy to store; nil leaves a value as it is. A target of
zero, an empty breach priority and the Clear switches remove a value.
*/
type Input struct {
	Name                   *string
	Description            *string
	ScheduleID             *ids.UUID
	ClearSchedule          bool
	FirstReply             *int32
	NextReply              *int32
	FollowUp               *int32
	Resolution             *int32
	WarnBefore             *int32
	PauseOnWaitingCustomer *bool
	PauseOnWaitingTeam     *bool
	BreachPriority         *string
	BreachTeamID           *ids.UUID
	ClearBreachTeam        bool
	TicketTypeIDs          []ids.UUID
	Priorities             []string
}

/*
canRead refuses a member who may not read conversations.
*/
func canRead(m authz.Membership) error {
	if !m.Can("conversations:read") && !m.Can("tickets:read") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
canManage refuses a member who may not set tickets up.
*/
func canManage(m authz.Membership) error {
	if !m.Can("tickets:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
List returns the workspace's policies, archived ones last.
*/
func (s *SLA) List(ctx context.Context, m authz.Membership) ([]models.SlaPolicy, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []models.SlaPolicy
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListSLAPolicies(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list sla policies: %w", err)
	}

	return out, nil
}

/*
Show returns one policy.
*/
func (s *SLA) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.SlaPolicy, error) {
	if err := canRead(m); err != nil {
		return models.SlaPolicy{}, err
	}

	var out models.SlaPolicy
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findPolicy(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Create stores a new policy.
*/
func (s *SLA) Create(ctx context.Context, m authz.Membership, in Input) (models.SlaPolicy, error) {
	if err := canManage(m); err != nil {
		return models.SlaPolicy{}, err
	}

	var out models.SlaPolicy
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		p := withInput(models.SlaPolicy{WarnBeforeSeconds: 900, PauseOnWaitingCustomer: true}, in)
		if err := check(ctx, q, m.Workspace.ID, p); err != nil {
			return err
		}

		var err error
		out, err = q.CreateSLAPolicy(ctx, models.CreateSLAPolicyParams{
			WorkspaceID: m.Workspace.ID, Name: p.Name, Description: p.Description, ScheduleID: p.ScheduleID,
			FirstReplySeconds: p.FirstReplySeconds, NextReplySeconds: p.NextReplySeconds,
			FollowUpSeconds: p.FollowUpSeconds, ResolutionSeconds: p.ResolutionSeconds,
			WarnBeforeSeconds: p.WarnBeforeSeconds, PauseOnWaitingCustomer: p.PauseOnWaitingCustomer,
			PauseOnWaitingTeam: p.PauseOnWaitingTeam, BreachPriority: p.BreachPriority,
			BreachTeamID: p.BreachTeamID, TicketTypeIds: p.TicketTypeIds, Priorities: p.Priorities,
			Now: s.Clock.Now(),
		})

		return err
	})

	return out, wrap("create sla policy", err)
}

/*
Update changes a policy. Running clocks keep the due times they have.
*/
func (s *SLA) Update(ctx context.Context, m authz.Membership, id ids.UUID, in Input) (models.SlaPolicy, error) {
	if err := canManage(m); err != nil {
		return models.SlaPolicy{}, err
	}

	var out models.SlaPolicy
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findPolicy(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		p := withInput(current, in)
		if err := check(ctx, q, m.Workspace.ID, p); err != nil {
			return err
		}

		out, err = q.UpdateSLAPolicy(ctx, models.UpdateSLAPolicyParams{
			Name: p.Name, Description: p.Description, ScheduleID: p.ScheduleID,
			FirstReplySeconds: p.FirstReplySeconds, NextReplySeconds: p.NextReplySeconds,
			FollowUpSeconds: p.FollowUpSeconds, ResolutionSeconds: p.ResolutionSeconds,
			WarnBeforeSeconds: p.WarnBeforeSeconds, PauseOnWaitingCustomer: p.PauseOnWaitingCustomer,
			PauseOnWaitingTeam: p.PauseOnWaitingTeam, BreachPriority: p.BreachPriority,
			BreachTeamID: p.BreachTeamID, TicketTypeIds: p.TicketTypeIds, Priorities: p.Priorities,
			Now: s.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return err
	})

	return out, wrap("update sla policy", err)
}

/*
Archive stops a policy from applying to new work and cancels its
running and paused clocks.
*/
func (s *SLA) Archive(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := canManage(m); err != nil {
		return err
	}

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		p, err := findPolicy(ctx, q, m.Workspace.ID, id)
		if err != nil || p.ArchivedAt != nil {
			return err
		}

		now := s.Clock.Now()
		if err := q.ArchiveSLAPolicy(ctx, models.ArchiveSLAPolicyParams{
			Now: &now, WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return fmt.Errorf("archive sla policy: %w", err)
		}

		return wrap("cancel policy clocks", q.CancelPolicyClocks(ctx, models.CancelPolicyClocksParams{
			Now: &now, WorkspaceID: m.Workspace.ID, PolicyID: id,
		}))
	})
}

/*
findPolicy returns one policy of the workspace, or ErrPolicyNotFound.
*/
func findPolicy(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.SlaPolicy, error) {
	p, err := q.FindSLAPolicy(ctx, models.FindSLAPolicyParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return p, ErrPolicyNotFound
	}

	if err != nil {
		return p, fmt.Errorf("find sla policy: %w", err)
	}

	return p, nil
}

/*
wrap names a failure that is not already one of ours.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
