/*
|--------------------------------------------------------------------------
| Offered Actions
|--------------------------------------------------------------------------
|
| Before the model writes, the run decides which actions it may call:
| published versions on active connections, allowed on the
| conversation's channel, and, for an action that binds the customer's
| identity, only where that identity is proven and every value it binds
| exists. The model never sees an action it could not run. Tools come
| sorted by name so the prompt stays cacheable, and each description
| says plainly whether the action only reads, and who agrees before a
| write runs.
|
*/

package actionkit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxOffered caps the actions offered in one run.
*/
const maxOffered = 20

/*
Live is a published action as a run offers it: its id, the version it
runs and that version's definition.
*/
type Live struct {
	ActionID  ids.UUID
	VersionID ids.UUID
	Spec      Spec
}

/*
OfferInput is what decides a run's actions: the workspace, the
conversation's channel, who the customer is, and whether writes may be
offered at all.
*/
type OfferInput struct {
	WorkspaceID ids.UUID
	ChannelType string
	Identity    Identity
	NoWrites    bool
}

/*
Offer returns the actions the run may call, sorted by name.
*/
func (k *Kit) Offer(ctx context.Context, in OfferInput) ([]Live, error) {
	var versions []models.AiActionVersion
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		versions, err = models.New(tx).ListLiveActionVersions(ctx, in.WorkspaceID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list live actions: %w", err)
	}

	contact := ContactValues(in.Identity)
	var out []Live
	for _, v := range versions {
		s, err := FromVersion(v)
		if err != nil || !offered(s, in, contact) {
			continue
		}
		out = append(out, Live{ActionID: v.ActionID, VersionID: v.ID, Spec: s})
	}
	slices.SortFunc(out, func(a, b Live) int { return strings.Compare(a.Spec.Name, b.Spec.Name) })
	if len(out) > maxOffered {
		out = out[:maxOffered]
	}
	return out, nil
}

/*
offered reports whether one action may be offered in the run.
*/
func offered(s Spec, in OfferInput, contact map[string]any) bool {
	switch {
	case !slices.Contains(s.Channels, in.ChannelType):
		return false
	case in.NoWrites && s.SideEffect == SideEffectWrite:
		return false
	case s.UsesContact() && !bindable(s, contact):
		return false
	}
	return true
}

/*
ToolSpec is how the model sees the action: its name, its description
with what it does to the account, and its input schema without the
arguments the admin bound.
*/
func (l Live) ToolSpec() contracts.ToolSpec {
	return contracts.ToolSpec{
		Name:        l.Spec.Name,
		Description: l.Spec.Description + " " + suffix(l.Spec),
		Schema:      l.Spec.ModelSchema(),
	}
}

/*
suffix tells the model what calling the action does.
*/
func suffix(s Spec) string {
	switch {
	case s.SideEffect == SideEffectRead:
		return "(Read-only lookup.)"
	case s.Confirmation == ConfirmationCustomer && s.Approval != ApprovalNone:
		return "(Changes the customer's account. Calling it is how you request it: the customer confirms in the " +
			"chat, then a teammate approves before anything runs. Do not ask for confirmation yourself and do not " +
			"hand off for it.)"
	case s.Confirmation == ConfirmationCustomer:
		return "(Changes the customer's account. The customer confirms in the chat before anything runs; " +
			"do not ask for confirmation yourself.)"
	case s.Approval != ApprovalNone:
		return "(Changes the customer's account. Calling it is how you ask the team: it sends the request to a " +
			"teammate for approval and nothing runs until they approve. Do not hand off for it; the result says the " +
			"request was sent and you tell the customer.)"
	}
	return "(Changes the customer's account and runs at once.)"
}
