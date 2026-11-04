package inbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
reply is a saved reply to store, by its owner, title and shortcut.
*/
type reply = models.CreateSavedReplyParams

/*
replies is a page of saved replies to list, by member and search.
*/
type replies = models.ListSavedRepliesParams

/*
saveReply stores the reply in the test's workspace, shared when it has
no owner, with a body and the clock's time.
*/
func saveReply(t *testing.T, tc *tests.TestCase, r reply) (models.SavedReply, error) {
	t.Helper()

	r.WorkspaceID, r.Body, r.Now = tc.Workspace.Workspace.ID, "Hi {{contact.first_name}}", tc.Clock.Now()

	return query(t, tc, r.WorkspaceID, func(ctx context.Context, q *models.Queries) (models.SavedReply, error) {
		return q.CreateSavedReply(ctx, r)
	})
}

/*
mustReply stores the reply and fails the test when it cannot.
*/
func mustReply(t *testing.T, tc *tests.TestCase, r reply) models.SavedReply {
	t.Helper()

	saved, err := saveReply(t, tc, r)
	require.NoError(t, err)

	return saved
}

/*
teammate adds a second member to the test's workspace.
*/
func teammate(t *testing.T, tc *tests.TestCase) models.WorkspaceMember {
	t.Helper()

	users, err := factories.User(t.Context(), tc.App)
	require.NoError(t, err)
	user, err := users.CreateOne(t.Context())
	require.NoError(t, err)
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
	})
	require.NoError(t, err)

	return member
}

/*
replyTitles walks every page the parameters ask for in the test's
workspace and returns the replies' titles in order.
*/
func replyTitles(t *testing.T, tc *tests.TestCase, params replies) []string {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID
	params.WorkspaceID = workspace

	var titles []string

	for {
		page, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.SavedReply, error) {
			return q.ListSavedReplies(ctx, params)
		})
		require.NoError(t, err)

		if len(page) == 0 {
			return titles
		}

		for _, r := range page {
			titles = append(titles, r.Title)
		}

		last := page[len(page)-1]
		params.AfterID, params.AfterTitle = &last.ID, &last.Title
	}
}

/*
byShortcut returns the reply the member reaches with the shortcut.
*/
func byShortcut(t *testing.T, tc *tests.TestCase, member ids.UUID, shortcut string) (models.SavedReply, error) {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID

	return query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.SavedReply, error) {
		return q.FindSavedReplyByShortcut(ctx, models.FindSavedReplyByShortcutParams{
			WorkspaceID: workspace, Shortcut: shortcut, MemberID: member,
		})
	})
}

func TestSavedReplyQueries(t *testing.T) {
	t.Parallel()

	t.Run("lists the shared replies and the member's own, by title", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner, mate := tc.Workspace.Owner.ID, teammate(t, tc).ID
		mustReply(t, tc, reply{Title: "Refund policy", Shortcut: new("refund")})
		mustReply(t, tc, reply{Title: "Greeting"})
		mustReply(t, tc, reply{OwnerMemberID: &owner, Title: "My signature", Shortcut: new("sig")})
		mustReply(t, tc, reply{OwnerMemberID: &mate, Title: "Their signature", Shortcut: new("sig")})

		ownersPaged := replyTitles(t, tc, replies{MemberID: owner, RowLimit: 2})
		mates := replyTitles(t, tc, replies{MemberID: mate, RowLimit: 10})
		searchedShortcut := replyTitles(t, tc, replies{MemberID: owner, Search: new("SIG"), RowLimit: 10})
		searchedTitle := replyTitles(t, tc, replies{MemberID: owner, Search: new("fund"), RowLimit: 10})

		assert.Equal(t, []string{"Greeting", "My signature", "Refund policy"}, ownersPaged)
		assert.Equal(t, []string{"Greeting", "Refund policy", "Their signature"}, mates)
		assert.Equal(t, []string{"My signature"}, searchedShortcut)
		assert.Equal(t, []string{"Refund policy"}, searchedTitle)
	})

	t.Run("keeps a shortcut to one reply per owner and one shared reply", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.Workspace.Owner.ID
		mustReply(t, tc, reply{Title: "Refund policy", Shortcut: new("refund")})
		mustReply(t, tc, reply{OwnerMemberID: &owner, Title: "My refund", Shortcut: new("refund")})
		mustReply(t, tc, reply{Title: "No shortcut"})

		_, shared := saveReply(t, tc, reply{Title: "Refund again", Shortcut: new("REFUND")})
		_, own := saveReply(t, tc, reply{OwnerMemberID: &owner, Title: "My refund again", Shortcut: new("refund")})
		_, loose := saveReply(t, tc, reply{Title: "Another without shortcut"})

		key := "saved_replies_workspace_id_owner_member_id_shortcut_key"
		assert.True(t, database.IsUniqueViolation(shared, key))
		assert.True(t, database.IsUniqueViolation(own, key))
		require.NoError(t, loose)
	})

	t.Run("finds a member's own reply by shortcut before the shared one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner, mate := tc.Workspace.Owner.ID, teammate(t, tc).ID
		shared := mustReply(t, tc, reply{Title: "Refund policy", Shortcut: new("refund")})
		own := mustReply(t, tc, reply{OwnerMemberID: &owner, Title: "My refund", Shortcut: new("refund")})

		ownFound, err := byShortcut(t, tc, owner, "Refund")
		require.NoError(t, err)
		sharedFound, err := byShortcut(t, tc, mate, "refund")
		require.NoError(t, err)
		_, err = byShortcut(t, tc, owner, "missing")

		assert.Equal(t, own, ownFound)
		assert.Equal(t, shared, sharedFound)
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("updates and deletes a reply in its workspace only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		saved := mustReply(t, tc, reply{Title: "Refund policy", Shortcut: new("refund")})
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		updated, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) (models.SavedReply, error) {
				return q.UpdateSavedReply(ctx, models.UpdateSavedReplyParams{
					WorkspaceID: workspace, ID: saved.ID, Title: "Refunds", Body: "We refund within 30 days.",
					Shortcut: nil, Now: tc.Clock.Now(),
				})
			})
		require.NoError(t, err)
		elsewhere, err := query(t, tc, other.Workspace.ID, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DeleteSavedReply(ctx, models.DeleteSavedReplyParams{WorkspaceID: workspace, ID: saved.ID})
		})
		require.NoError(t, err)
		found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.SavedReply, error) {
			return q.FindSavedReply(ctx, models.FindSavedReplyParams{WorkspaceID: workspace, ID: saved.ID})
		})
		require.NoError(t, err)
		deleted, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DeleteSavedReply(ctx, models.DeleteSavedReplyParams{WorkspaceID: workspace, ID: saved.ID})
		})
		require.NoError(t, err)

		assert.Equal(t, updated, found)
		assert.Equal(t, "Refunds", found.Title)
		assert.Nil(t, found.Shortcut)
		assert.Equal(t, []int64{0, 1}, []int64{elsewhere, deleted})
	})
}
