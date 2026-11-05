package conversations_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/drafts"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

func TestWorkbenchRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses every workbench action to a member holding no permission", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		acts := d.tc.App.Actions.Inbox
		ctx := tenancy.With(t.Context(), d.tc.Workspace.Workspace.ID.String())
		m := authz.NewMembership(d.tc.Workspace.Workspace, d.tc.Workspace.Owner, nil)
		view := conversationviews.Fields{Name: new("View"), Definition: json.RawMessage(`{}`)}
		saved := decode(t, d, models.PrefixConversationView,
			saveView(t, d.owner, "personal", oneCondition("status", "in", []string{"open"})))

		for label, call := range map[string]func() error{
			"snooze lists": func() error { _, err := acts.ListSnoozeLists.Handle(ctx, m); return err },
			"set snooze list": func() error {
				return acts.UpdateSnoozeList.Handle(ctx, conversations.UpdateSnoozeListInput{Membership: m})
			},
			"snooze choices": func() error {
				_, err := acts.ShowSnoozeOptions.Handle(ctx, conversations.SnoozeOptionsInput{Membership: m})
				return err
			},
			"preferences": func() error {
				_, err := acts.ShowPreferences.Handle(ctx, conversations.PreferencesInput{Membership: m})
				return err
			},
			"save preferences": func() error {
				_, err := acts.UpdatePreferences.Handle(ctx, conversations.UpdatePreferencesInput{Membership: m})
				return err
			},
			"views": func() error {
				_, err := acts.ListViews.Handle(ctx, conversationviews.ListInput{Membership: m})
				return err
			},
			"view": func() error {
				_, err := acts.ShowView.Handle(ctx, conversationviews.ShowInput{Membership: m, ID: ids.New()})
				return err
			},
			"create view": func() error {
				_, err := acts.CreateView.Handle(ctx, conversationviews.CreateInput{
					Membership: m, Scope: conversationviews.ScopePersonal, Fields: view,
				})
				return err
			},
			"change view": func() error {
				_, err := acts.UpdateView.Handle(ctx, conversationviews.UpdateInput{Membership: m, ID: saved})
				return err
			},
			"delete view": func() error {
				return acts.DeleteView.Handle(ctx, conversationviews.ShowInput{Membership: m, ID: saved})
			},
			"order views": func() error {
				_, err := acts.ReorderViews.Handle(ctx, conversationviews.ReorderInput{Membership: m})
				return err
			},
			"drafts": func() error {
				_, err := acts.ListDrafts.Handle(ctx, drafts.ListInput{Membership: m})
				return err
			},
			"save draft": func() error {
				_, err := acts.SaveDraft.Handle(ctx, drafts.SaveInput{Membership: m, Mode: conversations.DraftNew})
				return err
			},
			"change draft": func() error {
				_, err := acts.UpdateDraft.Handle(ctx, drafts.UpdateInput{Membership: m, ID: ids.New()})
				return err
			},
			"discard draft": func() error {
				return acts.DeleteDraft.Handle(ctx, drafts.DeleteInput{Membership: m, ID: ids.New()})
			},
			"delete message": func() error {
				_, err := acts.DeleteMessage.Handle(ctx, messages.UndoInput{Membership: m, MessageID: ids.New()})
				return err
			},
			"edit note": func() error {
				_, err := acts.EditNote.Handle(ctx, messages.EditInput{Membership: m, MessageID: ids.New()})
				return err
			},
			"list": func() error {
				_, err := acts.ListConversations.Handle(ctx, conversations.ListConversationsInput{Membership: m})
				return err
			},
		} {
			assert.ErrorIs(t, call(), exceptions.ErrForbidden, label)
		}
	})

	t.Run("refuses a view without its name or conditions", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := authz.NewMembership(d.tc.Workspace.Workspace, d.tc.Workspace.Owner, []string{"conversations:write"})

		_, err := d.tc.App.Actions.Inbox.CreateView.Handle(t.Context(), conversationviews.CreateInput{
			Membership: m, Scope: conversationviews.ScopePersonal,
		})
		assert.ErrorIs(t, err, exceptions.ErrInvalidDefinition)

		_, err = d.tc.App.Actions.Inbox.CreateView.Handle(t.Context(), conversationviews.CreateInput{
			Membership: m, Scope: "team", Fields: conversationviews.Fields{
				Name: new("View"), Definition: json.RawMessage(`{"op":"and","groups":[]}`),
			},
		})
		assert.ErrorIs(t, err, exceptions.ErrInvalidDefinition)
	})
}
