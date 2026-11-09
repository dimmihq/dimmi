/*
|--------------------------------------------------------------------------
| Triage On Arrival
|--------------------------------------------------------------------------
|
| A workspace may ask Operator to look at every conversation a customer
| starts. Operator reads it as the workspace's first owner would and
| proposes tags, an owner, a priority and a summary note, all as one
| changeset any teammate who sees the conversation reviews. It never
| applies anything on its own, since it read what the customer wrote,
| and it never drafts a reply. The run spends real tokens, which is why
| the setting is off until an admin turns it on.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Triage proposes how to file a conversation a customer just started,
when the workspace asked for it. A workspace that did not, or that
cannot run Operator now, is skipped without error.
*/
func (e *Engine) Triage(ctx context.Context, workspaceID, conversationID ids.UUID) error {
	e.prepare()
	s, err := e.openTriage(ctx, workspaceID, conversationID)
	if errors.Is(err, errSkip) {
		return nil
	}
	if err != nil {
		return err
	}
	_, status := e.converse(ctx, s)
	return e.DB.Transaction(context.WithoutCancel(ctx), func(ctx context.Context, tx *database.Tx) error {
		if err := e.finishRun(ctx, models.New(tx), s, status); err != nil {
			return err
		}
		return addSpend(ctx, tx, e, s)
	})
}

/*
errSkip marks a triage that should not run.
*/
var errSkip = errors.New("operator: skip triage")

/*
openTriage loads what a triage run needs, as the workspace's first
owner, and opens its run.
*/
func (e *Engine) openTriage(ctx context.Context, workspaceID, conversationID ids.UUID) (*session, error) {
	s := &session{background: true, question: TriageQuestion}
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		settings, err := LoadSettings(ctx, q, workspaceID)
		if err != nil {
			return err
		}
		if !settings.Enabled || !settings.TriageOnArrival {
			return errSkip
		}
		s.settings = settings
		if s.m, err = ownerMembership(ctx, q, workspaceID); err != nil {
			return err
		}
		c, err := conversations.Find(ctx, q, s.m, conversations.Ref{ID: conversationID})
		if err != nil {
			return errors.Join(errSkip, err)
		}
		s.conversation, s.thread.ConversationID = &c, &c.ID
		if err := e.loadModel(ctx, q, s); err != nil {
			return errors.Join(errSkip, err)
		}
		s.run, err = e.startRun(ctx, q, s)
		return err
	})
	if err != nil {
		return nil, err
	}
	if s.llm, err = e.LLM.ForKey(ctx, s.key); err != nil {
		return nil, fmt.Errorf("open operator model: %w", err)
	}
	return s, nil
}

/*
ownerMembership is the workspace's first active owner, whose view a
triage run reads with.
*/
func ownerMembership(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (authz.Membership, error) {
	ws, err := q.FindWorkspace(ctx, workspaceID)
	if err != nil {
		return authz.Membership{}, errors.Join(errSkip, err)
	}
	owners, err := q.ListActiveOwners(ctx, workspaceID)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("list owners: %w", err)
	}
	if len(owners) == 0 {
		return authz.Membership{}, errSkip
	}
	member := owners[0].WorkspaceMember
	return authz.NewMembership(ws, member, auth.RolePermissions(enums.Role(member.Role))), nil
}
