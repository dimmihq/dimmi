/*
|--------------------------------------------------------------------------
| Applying Changesets
|--------------------------------------------------------------------------
|
| A teammate applies, declines or reverts a changeset as one. Applying
| runs every operation through the action a teammate would use, as the
| teammate who decides, in one transaction: if one operation fails,
| nothing changed, and the changeset says why so it can be tried again.
| Reverting walks the operations backwards and puts back each value
| Operator replaced, skipping any field someone changed since rather
| than overwriting their work. Each decision lands in the audit log as
| the teammate's, made through Operator.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The changeset statuses.
*/
const (
	StatusPending  = "pending"
	StatusApplied  = "applied"
	StatusDeclined = "declined"
	StatusReverted = "reverted"
	StatusFailed   = "failed"
)

/*
The audit events of changesets, and the channel they are made through.
*/
const (
	EventChangesApplied  = "operator.changes_applied"
	EventChangesReverted = "operator.changes_reverted"
	ViaOperator          = "operator"
)

/*
ErrSettled refuses to decide a changeset that was already decided.
*/
var ErrSettled = fault.New(fault.Conflict, "changeset_settled", "This change was already decided.")

/*
Apply applies a pending changeset as the teammate deciding.
*/
func (e *Engine) Apply(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	e.prepare()
	var out models.OperatorChangeset
	var failure error
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := e.lockFor(ctx, q, m, wanted{id: id, statuses: []string{StatusPending, StatusFailed}})
		if err != nil {
			return err
		}
		ops := Operations(c.Operations)
		for i := range ops {
			if err := e.applyOp(ctx, m, &ops[i]); err != nil {
				failure = fmt.Errorf("%s: %w", ops[i].Label, err)
				return failure
			}
		}
		now := e.Clock.Now()
		out, err = settle(ctx, q, c, settlement{status: StatusApplied, ops: ops, by: m, appliedAt: &now, now: now})
		if err != nil {
			return err
		}
		return e.audit(ctx, m, out, EventChangesApplied)
	})
	if failure != nil {
		return e.fail(ctx, m, id, failure)
	}
	return out, err
}

/*
Revert undoes an applied changeset, newest operation first.
*/
func (e *Engine) Revert(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	e.prepare()
	var out models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := e.lockFor(ctx, q, m, wanted{id: id, statuses: []string{StatusApplied}})
		if err != nil {
			return err
		}
		ops := Operations(c.Operations)
		for i := len(ops) - 1; i >= 0; i-- {
			if err := e.revertOp(ctx, m, &ops[i]); err != nil {
				return fmt.Errorf("%s: %w", ops[i].Label, err)
			}
		}
		now := e.Clock.Now()
		out, err = settle(ctx, q, c, settlement{status: StatusReverted, ops: ops, by: m, revertedAt: &now, now: now})
		if err != nil {
			return err
		}
		return e.audit(ctx, m, out, EventChangesReverted)
	})
	return out, err
}

/*
Decline turns a changeset down; nothing changes.
*/
func (e *Engine) Decline(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	e.prepare()
	var out models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := e.lockFor(ctx, q, m, wanted{id: id, statuses: []string{StatusPending, StatusFailed}})
		if err != nil {
			return err
		}
		out, err = settle(ctx, q, c, settlement{
			status: StatusDeclined, ops: Operations(c.Operations), by: m, now: e.Clock.Now(),
		})
		return err
	})
	return out, err
}

/*
fail records why a changeset could not be applied and returns the
reason.
*/
func (e *Engine) fail(ctx context.Context, m authz.Membership, id ids.UUID, reason error,
) (models.OperatorChangeset, error) {
	var out models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := e.lockFor(ctx, q, m, wanted{id: id, statuses: []string{StatusPending, StatusFailed}})
		if err != nil {
			return err
		}
		out, err = settle(ctx, q, c, settlement{
			status: StatusFailed, ops: Operations(c.Operations), by: m, err: refusal(reason), now: e.Clock.Now(),
		})
		return err
	})
	return out, errors.Join(reason, err)
}

/*
wanted is the changeset to lock and the statuses it may be in.
*/
type wanted struct {
	id       ids.UUID
	statuses []string
}

/*
lockFor locks a changeset the teammate may decide, in one of the
statuses given: their own, or one Operator proposed on its own about a
conversation they can see.
*/
func (e *Engine) lockFor(ctx context.Context, q *models.Queries, m authz.Membership, w wanted,
) (models.OperatorChangeset, error) {
	c, err := q.LockOperatorChangeset(ctx, models.LockOperatorChangesetParams{WorkspaceID: m.Workspace.ID, ID: w.id})
	if errors.Is(err, database.ErrNotFound) {
		return c, exceptions.ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("lock changeset: %w", err)
	}
	if err := Visible(ctx, q, m, c); err != nil {
		return c, err
	}
	if !slices.Contains(w.statuses, c.Status) {
		return c, ErrSettled
	}
	return c, nil
}

/*
Visible refuses a changeset the teammate may not see: another
teammate's, or one about a conversation hidden from them.
*/
func Visible(ctx context.Context, q *models.Queries, m authz.Membership, c models.OperatorChangeset) error {
	if c.MemberID != nil && *c.MemberID != m.Member.ID {
		return exceptions.ErrNotFound
	}
	if c.MemberID == nil && c.ConversationID == nil {
		return exceptions.ErrNotFound
	}
	if c.ConversationID == nil {
		return nil
	}
	_, err := conversations.Find(ctx, q, m, conversations.Ref{ID: *c.ConversationID})
	return err
}

/*
settlement is what becomes of a changeset: its new status and
operations, who decided, when it was applied or reverted, and why it
failed.
*/
type settlement struct {
	status                string
	ops                   []Operation
	by                    authz.Membership
	appliedAt, revertedAt *time.Time
	err                   string
	now                   time.Time
}

/*
settle writes what became of a changeset.
*/
func settle(ctx context.Context, q *models.Queries, c models.OperatorChangeset, s settlement,
) (models.OperatorChangeset, error) {
	var reason *string
	if s.err != "" {
		reason = &s.err
	}
	out, err := q.SettleOperatorChangeset(ctx, models.SettleOperatorChangesetParams{
		WorkspaceID: c.WorkspaceID, ID: c.ID, Status: s.status, Operations: encodeOperations(s.ops),
		Error: reason, DecidedByMemberID: &s.by.Member.ID, AppliedAt: s.appliedAt, RevertedAt: s.revertedAt,
		Now: s.now,
	})
	if err != nil {
		return out, fmt.Errorf("settle changeset: %w", err)
	}
	return out, nil
}

/*
audit records a decision as the teammate's, made through Operator.
*/
func (e *Engine) audit(ctx context.Context, m authz.Membership, c models.OperatorChangeset, event string) error {
	if e.Services.Audit == nil {
		return nil
	}
	ops := Operations(c.Operations)
	changes := make([]map[string]any, len(ops))
	for i, op := range ops {
		changes[i] = map[string]any{
			"kind": op.Kind, "label": op.Label, "before": op.Before, "after": op.After, "outcome": op.Outcome,
		}
	}
	_, err := e.Services.Audit.Handle(ctx, audit.Entry{
		WorkspaceID: m.Workspace.ID, Event: event, ActorID: m.Member.ID, Via: ViaOperator,
		Subject: audit.Subject{Type: "operator_changeset", ID: c.ID, Label: c.Title},
		Changes: map[string]any{"operations": changes},
	})
	return err
}
