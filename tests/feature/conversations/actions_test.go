package conversations_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

func TestInboxActionsRefuseWithoutPermissions(t *testing.T) {
	t.Parallel()

	t.Run("refuses a caller who holds no inbox permission, whoever calls", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		acts := d.tc.App.Actions.Inbox
		m := authz.NewMembership(d.tc.Workspace.Workspace, d.tc.Workspace.Owner, nil)
		ref := conversations.Ref{Number: 1}
		ctx := t.Context()

		for name, call := range map[string]func(ctx context.Context) error{
			"list inboxes": func(ctx context.Context) error { _, err := acts.ListInboxes.Handle(ctx, m); return err },
			"show inbox": func(ctx context.Context) error {
				return second(acts.ShowInbox.Handle(ctx, inboxes.ShowInboxInput{Membership: m}))
			},
			"create inbox": func(ctx context.Context) error {
				return second(acts.CreateInbox.Handle(ctx, inboxes.CreateInboxInput{Membership: m}))
			},
			"update inbox": func(ctx context.Context) error {
				return second(acts.UpdateInbox.Handle(ctx, inboxes.UpdateInboxInput{Membership: m}))
			},
			"list contacts": func(ctx context.Context) error {
				return second(acts.ListContacts.Handle(ctx, contacts.ListContactsInput{Membership: m}))
			},
			"show contact": func(ctx context.Context) error {
				return second(acts.ShowContact.Handle(ctx, contacts.ShowContactInput{Membership: m}))
			},
			"create contact": func(ctx context.Context) error {
				return second(acts.CreateContact.Handle(ctx, contacts.CreateContactInput{Membership: m}))
			},
			"update contact": func(ctx context.Context) error {
				return second(acts.UpdateContact.Handle(ctx, contacts.UpdateContactInput{Membership: m}))
			},
			"list conversations": func(ctx context.Context) error {
				return second(acts.ListConversations.Handle(ctx, conversations.ListConversationsInput{Membership: m}))
			},
			"show conversation": func(ctx context.Context) error {
				in := conversations.ShowConversationInput{Membership: m, Ref: ref}

				return second(acts.ShowConversation.Handle(ctx, in))
			},
			"create conversation": func(ctx context.Context) error {
				return second(acts.CreateConversation.Handle(ctx, conversations.CreateConversationInput{Membership: m}))
			},
			"apply changes": func(ctx context.Context) error {
				return second(acts.ApplyChanges.Handle(ctx, conversations.ApplyChangesInput{Membership: m, Ref: ref}))
			},
			"neighbors": func(ctx context.Context) error {
				return second(acts.ShowNeighbors.Handle(ctx, conversations.NeighborsInput{Membership: m, Ref: ref}))
			},
			"counts": func(ctx context.Context) error {
				return second(acts.CountConversations.Handle(ctx, conversations.CountsInput{Membership: m}))
			},
			"show read state": func(ctx context.Context) error {
				return second(acts.ShowReadState.Handle(ctx, conversations.ReadStateInput{Membership: m, Ref: ref}))
			},
			"update read state": func(ctx context.Context) error {
				in := conversations.UpdateReadStateInput{Membership: m, Ref: ref}

				return second(acts.UpdateReadState.Handle(ctx, in))
			},
			"search": func(ctx context.Context) error {
				return second(acts.Search.Handle(ctx, conversations.SearchInput{Membership: m}))
			},
			"list messages": func(ctx context.Context) error {
				return second(acts.ListMessages.Handle(ctx, messages.ListMessagesInput{Membership: m, Ref: ref}))
			},
			"reply": func(ctx context.Context) error {
				return second(acts.Reply.Handle(ctx, messages.ReplyInput{Membership: m, Ref: ref}))
			},
			"note": func(ctx context.Context) error {
				return second(acts.AddNote.Handle(ctx, messages.NoteInput{Membership: m, Ref: ref}))
			},
			"contact message": func(ctx context.Context) error {
				return second(acts.AddContactMessage.Handle(ctx, messages.ContactMessageInput{Membership: m, Ref: ref}))
			},
		} {
			require.ErrorIs(t, call(ctx), exceptions.ErrForbidden, name)
		}
	})
}

/*
second returns the error of a call that also returns a value.
*/
func second[T any](_ T, err error) error {
	return err
}
