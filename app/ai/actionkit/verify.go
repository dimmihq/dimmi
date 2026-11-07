/*
|--------------------------------------------------------------------------
| Calls Waiting For An Email Proof
|--------------------------------------------------------------------------
|
| An anonymous messenger visitor can still use an action bound only to
| their email: the call waits with its arguments frozen and nothing
| bound, the visitor confirms their address with a code typed into a
| card, and only then is the proven address bound. From there the call
| goes on as if just proposed: to the customer's confirmation, to the
| team's approval, or straight to execution, whose result the agent
| reports. The model never supplies the address and never sees the code.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
onlyEmail reports whether the only customer value the action binds is
their email.
*/
func onlyEmail(s Spec) bool {
	found := false
	for _, r := range s.Refs() {
		if !r.UsesContact() {
			continue
		}
		if r.Path[0] != actiontemplate.ContactEmail {
			return false
		}
		found = true
	}
	return found
}

/*
awaitsEmail reports whether the action can wait for the visitor to
prove their email instead of being refused.
*/
func awaitsEmail(s Spec, id Identity) bool {
	return id.EmailCodes && onlyEmail(s)
}

/*
awaitEmail records a call that waits for the visitor's email proof.
*/
func (k *Kit) awaitEmail(ctx context.Context, in CallInput, input map[string]any) (CallReply, error) {
	nc := newCall{
		input: input, bindings: Bindings{Contact: map[string]any{}}, state: "awaiting_verification",
		expires: k.Clock.Now().Add(ConfirmationTTL),
	}
	var call models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := k.supersede(ctx, tx, in); err != nil {
			return err
		}
		row, err := k.insert(ctx, tx, in, nc)
		if err != nil {
			return err
		}
		call = row
		if err := k.note(ctx, tx, step{call, in.Agent, EventRequested}); err != nil {
			return err
		}
		return k.scheduleExpiry(ctx, tx, call)
	})
	if err != nil {
		return CallReply{}, err
	}
	return CallReply{Text: pendingText(in.Live.Spec, call), Call: &call, Awaiting: true}, nil
}

/*
EmailProven binds the contact's calls waiting for an email proof to the
proven address and moves each on.
*/
func (k *Kit) EmailProven(ctx context.Context, workspaceID, contactID ids.UUID, email string) error {
	return k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		calls, err := q.ListCallsAwaitingEmail(ctx, models.ListCallsAwaitingEmailParams{
			WorkspaceID: workspaceID, ContactID: &contactID, Now: k.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("list calls awaiting email: %w", err)
		}
		for _, call := range calls {
			if err := k.bindEmail(ctx, tx, call, email); err != nil {
				return err
			}
		}
		return nil
	})
}

/*
bindEmail binds one call and sends it where a new call of its action
would go.
*/
func (k *Kit) bindEmail(ctx context.Context, tx *database.Tx, call models.AiActionCall, email string) error {
	bindings, err := json.Marshal(Bindings{Contact: map[string]any{actiontemplate.ContactEmail: email}})
	if err != nil {
		return fmt.Errorf("encode bindings: %w", err)
	}
	q, now := models.New(tx), k.Clock.Now()
	next, expires := "approved", (*time.Time)(nil)
	switch {
	case call.Confirmation == ConfirmationCustomer:
		next, expires = "awaiting_confirmation", new(now.Add(ConfirmationTTL))
	case call.Approval != ApprovalNone:
		s, err := specOf(ctx, q, call)
		if err != nil {
			return err
		}
		next, expires = "pending_approval", new(now.Add(approvalTTL(s)))
	}
	row, err := q.BindVerifiedActionCall(ctx, models.BindVerifiedActionCallParams{
		Bindings: bindings, NextState: next, ExpiresAt: expires, Now: now, WorkspaceID: call.WorkspaceID, ID: call.ID,
	})
	if err != nil {
		return fmt.Errorf("bind verified call: %w", err)
	}
	switch row.State {
	case "approved":
		return k.run(ctx, tx, row)
	case "pending_approval":
		if err := k.askApproval(ctx, tx, row); err != nil {
			return err
		}
	}
	return k.scheduleExpiry(ctx, tx, row)
}
