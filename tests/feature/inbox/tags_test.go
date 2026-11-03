package inbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
ensureTags returns the workspace's tags with the names, creating those
missing.
*/
func ensureTags(t *testing.T, tc *tests.TestCase, workspace ids.UUID, names ...string) []models.EnsureTagsRow {
	t.Helper()

	tags, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.EnsureTagsRow, error) {
		return q.EnsureTags(ctx, models.EnsureTagsParams{WorkspaceID: workspace, Names: names, Now: tc.Clock.Now()})
	})
	require.NoError(t, err)

	return tags
}

/*
tagNames returns the names of the tags, in order.
*/
func tagNames[T models.Tag | models.EnsureTagsRow](tags []T) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, models.Tag(tag).Name)
	}

	return names
}

/*
tag puts the tags with the ids on the conversation, as its owner, and
returns the ids it added.
*/
func tag(t *testing.T, tc *tests.TestCase, c models.Conversation, tagIDs ...ids.UUID) []ids.UUID {
	t.Helper()

	added, err := query(t, tc, c.WorkspaceID, func(ctx context.Context, q *models.Queries) ([]ids.UUID, error) {
		return q.TagConversation(ctx, models.TagConversationParams{
			WorkspaceID: c.WorkspaceID, ConversationID: c.ID, TagIds: tagIDs, CreatedByType: "member",
			CreatedByID: &tc.Workspace.Owner.ID, Now: tc.Clock.Now(),
		})
	})
	require.NoError(t, err)

	return added
}

/*
listTags returns the names of the workspace's tags starting with the
prefix, or all of them.
*/
func listTags(t *testing.T, tc *tests.TestCase, prefix *string) []string {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID
	tags, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Tag, error) {
		return q.ListTags(ctx, models.ListTagsParams{WorkspaceID: workspace, Prefix: prefix})
	})
	require.NoError(t, err)

	return tagNames(tags)
}

func TestTagQueries(t *testing.T) {
	t.Parallel()

	t.Run("creates missing tags by name once, without regard to case", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID

		first := ensureTags(t, tc, workspace, "billing", "Bug")
		again := ensureTags(t, tc, workspace, "BILLING", "vip", "Vip", "bug")
		_, duplicate := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Tag, error) {
			return q.CreateTag(ctx, models.CreateTagParams{
				WorkspaceID: workspace, Name: "Billing", Now: tc.Clock.Now(),
			})
		})

		assert.Equal(t, []string{"billing", "Bug"}, tagNames(first))
		assert.Equal(t, []string{"billing", "Bug", "vip"}, tagNames(again))
		assert.Equal(t, first[0].ID, again[0].ID)
		assert.True(t, database.IsUniqueViolation(duplicate, "tags_workspace_id_name_key"))
		assert.Equal(t, []string{"billing", "Bug", "vip"}, listTags(t, tc, nil))
		assert.Equal(t, []string{"billing", "Bug"}, listTags(t, tc, new("B")))
	})

	t.Run("finds, renames and deletes a tag in its workspace only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		created := ensureTags(t, tc, workspace, "billing")[0]
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		renamed, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Tag, error) {
			return q.UpdateTag(ctx, models.UpdateTagParams{
				WorkspaceID: workspace, ID: created.ID, Name: "payments", Color: new("#f59e0b"), Now: tc.Clock.Now(),
			})
		})
		require.NoError(t, err)
		elsewhere, err := query(t, tc, other.Workspace.ID, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DeleteTag(ctx, models.DeleteTagParams{WorkspaceID: workspace, ID: created.ID})
		})
		require.NoError(t, err)
		found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Tag, error) {
			return q.FindTag(ctx, models.FindTagParams{WorkspaceID: workspace, ID: created.ID})
		})
		require.NoError(t, err)
		deleted, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DeleteTag(ctx, models.DeleteTagParams{WorkspaceID: workspace, ID: created.ID})
		})
		require.NoError(t, err)

		assert.Equal(t, renamed, found)
		assert.Equal(t, "payments", found.Name)
		assert.Equal(t, "#f59e0b", *found.Color)
		assert.Equal(t, int64(0), elsewhere)
		assert.Equal(t, int64(1), deleted)
		assert.Empty(t, listTags(t, tc, nil))
	})

	t.Run("tags a conversation once per tag and never with another workspace's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		ours := ensureTags(t, tc, workspace, "billing", "vip")
		theirs := ensureTags(t, tc, other.Workspace.ID, "foreign")[0]

		first := tag(t, tc, conversation, ours[0].ID, theirs.ID)
		second := tag(t, tc, conversation, ours[0].ID, ours[1].ID)
		count, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.CountConversationTags(ctx, models.CountConversationTagsParams{
				WorkspaceID: workspace, ConversationID: conversation.ID,
			})
		})
		require.NoError(t, err)

		assert.Equal(t, []ids.UUID{ours[0].ID}, first)
		assert.Equal(t, []ids.UUID{ours[1].ID}, second)
		assert.Equal(t, int64(2), count)
	})

	t.Run("loads the tags of a page of conversations and counts tagged conversations", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		refund, upgrade := startConversation(t, tc, th), startConversation(t, tc, th)
		deleted := startConversation(t, tc, th)
		tags := ensureTags(t, tc, workspace, "billing", "vip")
		billing, vip := tags[0], tags[1]
		tag(t, tc, refund, vip.ID, billing.ID)
		tag(t, tc, upgrade, billing.ID)
		tag(t, tc, deleted, billing.ID)
		exec(t, tc, "UPDATE conversations SET deleted_at = now() WHERE id = $1", deleted.ID)

		rows, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.ListConversationTagsRow, error) {
				return q.ListConversationTags(ctx, models.ListConversationTagsParams{
					WorkspaceID: workspace, ConversationIds: []ids.UUID{refund.ID, upgrade.ID},
				})
			})
		require.NoError(t, err)
		counts, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.CountTaggedConversationsRow, error) {
				return q.CountTaggedConversations(ctx, workspace)
			})
		require.NoError(t, err)

		byConversation := map[ids.UUID][]string{}
		for _, row := range rows {
			byConversation[row.ConversationID] = append(byConversation[row.ConversationID], row.Tag.Name)
		}

		perTag := map[ids.UUID]int64{}
		for _, row := range counts {
			perTag[row.TagID] = row.Conversations
		}

		assert.Equal(t, map[ids.UUID][]string{refund.ID: {"billing", "vip"}, upgrade.ID: {"billing"}}, byConversation)
		assert.Equal(t, map[ids.UUID]int64{billing.ID: 2, vip.ID: 1}, perTag)
	})

	t.Run("takes a tag off a conversation once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		billing := ensureTags(t, tc, workspace, "billing")[0]
		tag(t, tc, conversation, billing.ID)
		untag := func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.UntagConversation(ctx, models.UntagConversationParams{
				WorkspaceID: workspace, ConversationID: conversation.ID, TagID: billing.ID,
			})
		}

		removed, err := query(t, tc, workspace, untag)
		require.NoError(t, err)
		again, err := query(t, tc, workspace, untag)
		require.NoError(t, err)

		assert.Equal(t, []int64{1, 0}, []int64{removed, again})
		assert.Equal(t, []string{"billing"}, listTags(t, tc, nil))
	})
}
