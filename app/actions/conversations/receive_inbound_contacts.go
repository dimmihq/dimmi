/*
|--------------------------------------------------------------------------
| Receive Inbound Message: Contacts
|--------------------------------------------------------------------------
|
| Who a message is from, and who else is on its thread, as contacts. An
| identity the workspace knows finds its contact; a new one creates a
| lead. A proven address marks the contact's address verified, which
| an unproven one never does, so nobody can borrow a verified
| customer's history by typing their address. A display name only
| fills a contact's name when it has none: a teammate's edit always
| stands.
|
| People copied on a mail join its conversation as copies; someone
| answering a thread they were not on joins as a participant. Neither
| ever replaces the conversation's own contact.
|
*/

package conversations

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
resolveContact returns the contact a sender is: the one the channel names
when it already knows, otherwise the one an identity finds, creating a lead
for a sender nobody knows.
*/
func (run *ingestRun) resolveContact(ctx context.Context, s contracts.InboundSender) (models.Contact, error) {
	if s.ContactID != nil {
		contact, err := run.q.FindContact(ctx, models.FindContactParams{WorkspaceID: run.ws, ID: *s.ContactID})
		if err != nil {
			return contact, fmt.Errorf("find sender: %w", err)
		}

		return run.known(ctx, contact, s, contracts.IdentityClaim{})
	}

	if err := run.lockIdentities(ctx, s.Identities); err != nil {
		return models.Contact{}, err
	}

	for _, claim := range s.Identities {
		contact, err := run.q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
			WorkspaceID: run.ws, Type: string(claim.Type), Value: claim.Value,
		})
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		if err != nil {
			return contact, fmt.Errorf("find contact: %w", err)
		}

		return run.known(ctx, contact, s, claim)
	}

	return run.create(ctx, s)
}

/*
lockIdentities holds the sender's identities, in one order so two
senders never wait on each other, until the message is stored, so a
second message from the same new sender finds the contact the first
created instead of creating it again.
*/
func (run *ingestRun) lockIdentities(ctx context.Context, claims []contracts.IdentityClaim) error {
	keys := make([]contracts.IdentityClaim, len(claims))
	copy(keys, claims)
	slices.SortFunc(keys, func(a, b contracts.IdentityClaim) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Value, b.Value))
	})

	for _, claim := range keys {
		err := run.q.LockContactIdentity(ctx, models.LockContactIdentityParams{
			WorkspaceID: run.ws.String(), Type: string(claim.Type), Value: claim.Value,
		})
		if err != nil {
			return fmt.Errorf("lock contact identity: %w", err)
		}
	}

	return nil
}

/*
known follows a merged contact to the one it became, proves the
sender's address when the claim is verified, and notes that they wrote.
*/
func (run *ingestRun) known(
	ctx context.Context, contact models.Contact, s contracts.InboundSender, claim contracts.IdentityClaim,
) (models.Contact, error) {
	if contact.MergedIntoID != nil {
		merged, err := run.q.FindContact(ctx, models.FindContactParams{WorkspaceID: run.ws, ID: *contact.MergedIntoID})
		if err != nil {
			return contact, fmt.Errorf("find merged contact: %w", err)
		}

		contact = merged
	}

	if claim.Verified && claim.Type == enums.IdentityTypeEmail {
		err := run.q.VerifyEmailContactIdentity(ctx, models.VerifyEmailContactIdentityParams{
			Now: run.r.Clock.Now(), WorkspaceID: run.ws, ContactID: contact.ID, Email: &claim.Value,
		})
		if err != nil {
			return contact, fmt.Errorf("verify contact address: %w", err)
		}
	}

	err := run.q.SeeEmailContact(ctx, models.SeeEmailContactParams{
		Name: optional(s.Name), Now: run.r.Clock.Now(), WorkspaceID: run.ws, ID: contact.ID,
	})
	if err != nil {
		return contact, fmt.Errorf("note contact seen: %w", err)
	}

	return contact, nil
}

/*
create stores a new lead with the sender's identities.
*/
func (run *ingestRun) create(ctx context.Context, s contracts.InboundSender) (models.Contact, error) {
	verified := false
	for _, claim := range s.Identities {
		verified = verified || (claim.Type == enums.IdentityTypeEmail && claim.Verified)
	}

	kind := s.Kind
	if kind == "" {
		kind = enums.ContactKindLead
	}

	contact, err := run.q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: run.ws, Kind: string(kind), Email: optional(s.Email), EmailVerified: verified && s.Email != "",
		Name: optional(s.Name), CustomAttributes: []byte(`{}`), Now: run.r.Clock.Now(),
	})
	if err != nil {
		return contact, fmt.Errorf("create contact: %w", err)
	}

	for _, claim := range s.Identities {
		_, err := run.q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
			WorkspaceID: run.ws, ContactID: contact.ID, Type: string(claim.Type), Value: claim.Value,
			Verified: claim.Verified, Now: run.r.Clock.Now(),
		})
		if err != nil {
			return contact, fmt.Errorf("create contact identity: %w", err)
		}
	}

	return contact, nil
}

/*
join puts the people copied on the message on the conversation, and the
sender too when they are not its contact.
*/
func (run *ingestRun) join(ctx context.Context, conv models.Conversation, sender models.Contact) error {
	if sender.ID != conv.ContactID {
		if err := run.participate(ctx, conv, sender, enums.ParticipantRoleParticipant); err != nil {
			return err
		}
	}

	for _, person := range run.msg.Participants {
		contact, err := run.resolveContact(ctx, person)
		if err != nil {
			return err
		}

		if contact.ID == conv.ContactID {
			continue
		}

		if err := run.participate(ctx, conv, contact, enums.ParticipantRoleCC); err != nil {
			return err
		}
	}

	return nil
}

/*
participate puts the contact on the conversation, keeping the role of
one already on it.
*/
func (run *ingestRun) participate(
	ctx context.Context, conv models.Conversation, contact models.Contact, role enums.ParticipantRole,
) error {
	_, err := run.q.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
		WorkspaceID: run.ws, ConversationID: conv.ID, ContactID: contact.ID, Role: string(role),
		Now: run.r.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("add participant: %w", err)
	}

	return nil
}
