package inbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
upload stores an unattached upload in the workspace, made by the
workspace's owner.
*/
func upload(t *testing.T, tc *tests.TestCase, workspace ids.UUID, filename string) models.Attachment {
	t.Helper()

	attachment, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
		return q.CreateAttachment(ctx, models.CreateAttachmentParams{
			WorkspaceID: workspace, UploadedByType: "member", UploadedByID: &tc.Workspace.Owner.ID,
			Disk: "local", Path: "uploads/" + filename, Filename: filename, ContentType: "application/pdf",
			SizeBytes: 48211, ScanStatus: "pending", Now: tc.Clock.Now(),
		})
	})
	require.NoError(t, err)

	return attachment
}

/*
attachmentIDs returns the ids of the attachments, in order.
*/
func attachmentIDs(attachments []models.Attachment) []ids.UUID {
	out := make([]ids.UUID, 0, len(attachments))
	for _, a := range attachments {
		out = append(out, a.ID)
	}

	return out
}

/*
link attaches the uploads to the message and returns how many it
attached.
*/
func link(t *testing.T, tc *tests.TestCase, m models.Message, uploads ...models.Attachment) int64 {
	t.Helper()

	linked, err := query(t, tc, m.WorkspaceID, func(ctx context.Context, q *models.Queries) (int64, error) {
		return q.LinkAttachments(ctx, models.LinkAttachmentsParams{
			WorkspaceID: m.WorkspaceID, MessageID: &m.ID, Ids: attachmentIDs(uploads), Now: tc.Clock.Now(),
		})
	})
	require.NoError(t, err)

	return linked
}

func TestAttachmentQueries(t *testing.T) {
	t.Parallel()

	t.Run("links uploads to one message only, never another workspace's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		first, err := appendMessage(t, tc, message(conversation, "Receipt attached", tc.Clock.Now()))
		require.NoError(t, err)
		second, err := appendMessage(t, tc, message(conversation, "And another", tc.Clock.Now()))
		require.NoError(t, err)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		receipt, invoice, foreign := upload(t, tc, workspace, "receipt.pdf"), upload(t, tc, workspace, "invoice.pdf"),
			upload(t, tc, other.Workspace.ID, "foreign.pdf")

		checked, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.Attachment, error) {
				return q.ListAttachmentsByID(ctx, models.ListAttachmentsByIDParams{
					WorkspaceID: workspace, Ids: []ids.UUID{receipt.ID, foreign.ID},
				})
			})
		require.NoError(t, err)

		assert.Equal(t, []models.Attachment{receipt}, checked)
		assert.Equal(t, int64(2), link(t, tc, first, receipt, invoice, foreign))
		assert.Equal(t, int64(0), link(t, tc, second, receipt))

		listed, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.Attachment, error) {
				return q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
					WorkspaceID: workspace, MessageIds: []ids.UUID{first.ID, second.ID},
				})
			})
		require.NoError(t, err)

		assert.Equal(t, []ids.UUID{receipt.ID, invoice.ID}, attachmentIDs(listed))
		assert.Equal(t, &first.ID, listed[0].MessageID)
		assert.Equal(t, "pending", listed[0].ScanStatus)
	})

	t.Run("records the scan and deletes only an upload no message uses", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		sent, err := appendMessage(t, tc, message(conversation, "Receipt attached", tc.Clock.Now()))
		require.NoError(t, err)
		attached, loose := upload(t, tc, workspace, "receipt.pdf"), upload(t, tc, workspace, "draft.pdf")
		link(t, tc, sent, attached)

		scanned, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
				return q.SetAttachmentScanStatus(ctx, models.SetAttachmentScanStatusParams{
					WorkspaceID: workspace, ID: attached.ID, ScanStatus: "clean", Now: tc.Clock.Now(),
				})
			})
		require.NoError(t, err)
		deleted, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]int64, error) {
			kept, err := q.DeleteUnattachedAttachment(ctx, models.DeleteUnattachedAttachmentParams{
				WorkspaceID: workspace, ID: attached.ID,
			})
			if err != nil {
				return nil, err
			}

			gone, err := q.DeleteUnattachedAttachment(ctx, models.DeleteUnattachedAttachmentParams{
				WorkspaceID: workspace, ID: loose.ID,
			})

			return []int64{kept, gone}, err
		})
		require.NoError(t, err)
		_, err = query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
			return q.FindAttachment(ctx, models.FindAttachmentParams{WorkspaceID: workspace, ID: loose.ID})
		})

		assert.Equal(t, "clean", scanned.ScanStatus)
		assert.Equal(t, []int64{0, 1}, deleted)
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("lists the uploads left unattached past the cutoff, oldest first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		sent, err := appendMessage(t, tc, message(conversation, "Receipt attached", tc.Clock.Now()))
		require.NoError(t, err)
		old, older := upload(t, tc, workspace, "old.pdf"), upload(t, tc, workspace, "older.pdf")
		link(t, tc, sent, upload(t, tc, workspace, "sent.pdf"))
		tc.Clock.Travel(25 * time.Hour)
		upload(t, tc, workspace, "fresh.pdf")

		stale, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.Attachment, error) {
				return q.ListUnattachedAttachments(ctx, models.ListUnattachedAttachmentsParams{
					WorkspaceID: workspace, Before: tc.Clock.Now().Add(-24 * time.Hour), RowLimit: 10,
				})
			})
		require.NoError(t, err)

		assert.Equal(t, []ids.UUID{old.ID, older.ID}, attachmentIDs(stale))
	})
}
