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
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxOffered caps the actions offered in one run; actions a procedure
pins may raise it to maxPinned.
*/
const (
	maxOffered = 20
	maxPinned  = 30
)

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
conversation's channel, who the customer is, whether writes may be
offered at all, and the agent asking, dimmi when empty. Operator works
for the team, so the customer's channel does not limit its actions.
MustInclude pins the actions a procedure the run follows takes: each
one the run may call is offered first, whatever the cap. Scope is
which actions the answering agent may call; its zero value allows all.
*/
type OfferInput struct {
	WorkspaceID ids.UUID
	ChannelType string
	Identity    Identity
	NoWrites    bool
	Agent       string
	MustInclude []ids.UUID
	Scope       agents.Scope
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
	var pinned, other []Live
	for _, v := range versions {
		s, err := FromVersion(v)
		if err != nil || !in.Scope.Allows(v.ActionID) || !offered(s, in, contact) {
			continue
		}
		live := Live{ActionID: v.ActionID, VersionID: v.ID, Spec: s}
		if slices.Contains(in.MustInclude, v.ActionID) {
			pinned = append(pinned, live)
			continue
		}
		other = append(other, live)
	}
	return pin(pinned, other), nil
}

/*
pin keeps every pinned action, up to maxPinned, and fills the rest of
the cap with the others by name; the result is sorted by name so the
prompt stays cacheable.
*/
func pin(pinned, other []Live) []Live {
	byName := func(a, b Live) int { return strings.Compare(a.Spec.Name, b.Spec.Name) }
	slices.SortFunc(pinned, byName)
	slices.SortFunc(other, byName)
	if len(pinned) > maxPinned {
		pinned = pinned[:maxPinned]
	}
	room := max(maxOffered, len(pinned)) - len(pinned)
	out := slices.Concat(pinned, other[:min(room, len(other))])
	slices.SortFunc(out, byName)
	return out
}

/*
Unavailable says why an action cannot be offered in the run, empty when
it can: identity_unverified, channel_unsupported, writes_held or
connection_disabled when it is not live at all.
*/
func (k *Kit) Unavailable(ctx context.Context, in OfferInput, actionID ids.UUID) string {
	var versions []models.AiActionVersion
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		versions, err = models.New(tx).ListLiveActionVersions(ctx, in.WorkspaceID)
		return err
	})
	if err != nil {
		return "connection_disabled"
	}
	for _, v := range versions {
		if v.ActionID != actionID {
			continue
		}
		s, err := FromVersion(v)
		if err != nil {
			return "connection_disabled"
		}
		return unavailableReason(s, in, ContactValues(in.Identity))
	}
	return "connection_disabled"
}

/*
unavailableReason names why one action cannot be offered.
*/
func unavailableReason(s Spec, in OfferInput, contact map[string]any) string {
	agent := cmp.Or(in.Agent, AgentDimmi)
	switch {
	case !s.UsableBy(agent):
		return "connection_disabled"
	case agent == AgentDimmi && !slices.Contains(s.Channels, in.ChannelType):
		return "channel_unsupported"
	case in.NoWrites && s.SideEffect == SideEffectWrite:
		return "writes_held"
	case s.UsesContact() && !bindable(s, contact) && !awaitsEmail(s, in.Identity):
		return "identity_unverified"
	}
	return ""
}

/*
offered reports whether one action may be offered in the run.
*/
func offered(s Spec, in OfferInput, contact map[string]any) bool {
	agent := cmp.Or(in.Agent, AgentDimmi)
	switch {
	case !s.UsableBy(agent):
		return false
	case agent == AgentDimmi && !slices.Contains(s.Channels, in.ChannelType):
		return false
	case in.NoWrites && s.SideEffect == SideEffectWrite:
		return false
	case s.UsesContact() && !bindable(s, contact):
		return awaitsEmail(s, in.Identity)
	}
	return true
}

/*
ToolSpec is how the model sees the action: its name, its description
with what it does to the account, and its input schema without the
arguments the admin bound.
*/
func (l Live) ToolSpec() contracts.ToolSpec {
	description := l.Spec.Description + " " + suffix(l.Spec)
	if onlyEmail(l.Spec) {
		description += " " + emailNote
	}
	return contracts.ToolSpec{Name: l.Spec.Name, Description: description, Schema: l.Spec.ModelSchema()}
}

/*
emailNote is what the model reads about actions bound to the customer's
email, which an anonymous visitor confirms with a code.
*/
const emailNote = "(Uses the customer's email. If they have not confirmed it, calling it shows them a card to " +
	"confirm their email with a code first; never ask them for the code or their email in the chat.)"

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
