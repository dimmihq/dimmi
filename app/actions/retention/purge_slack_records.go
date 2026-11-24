/*
|--------------------------------------------------------------------------
| Purge Slack Records
|--------------------------------------------------------------------------
|
| The parts of the Slack purge that are not conversations: messages that
| traveled on the connection in conversations from elsewhere, such as
| after a merge, and what the connection itself kept, which are the
| deliveries, message ids, triage posts, teammate links, talks with
| Operator and rooms.
|
*/

package retention

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
strayMessages empties the messages that traveled on the connection in
conversations that came from elsewhere, a batch at a time.
*/
func (a *PurgeSlackData) strayMessages(ctx context.Context, p *slackPurge) error {
	for {
		found, files, err := a.strays(ctx, p)
		if err != nil || len(found) == 0 {
			return err
		}

		if err := deleteStoredFiles(ctx, a.Storage, files); err != nil {
			return err
		}

		if err := a.wipe(ctx, p, found); err != nil {
			return err
		}

		if len(found) < purgeBatch {
			return nil
		}
	}
}

/*
strays returns a batch of such messages that still hold something, and
their stored files.
*/
func (a *PurgeSlackData) strays(ctx context.Context, p *slackPurge) ([]ids.UUID, []storedFile, error) {
	var (
		found []ids.UUID
		files []storedFile
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), p.conn.WorkspaceID

		var err error

		found, err = q.ListConnectionStrayMessages(ctx, models.ListConnectionStrayMessagesParams{
			WorkspaceID: ws, ChannelConnectionID: &p.conn.ID, RowLimit: purgeBatch,
		})
		if err != nil {
			return fmt.Errorf("list messages: %w", err)
		}

		rows, err := q.ListSlackMessageFiles(ctx, models.ListSlackMessageFilesParams{
			WorkspaceID: ws, MessageIds: found,
		})
		if err != nil {
			return fmt.Errorf("list attachments: %w", err)
		}

		for _, row := range rows {
			files = append(files, storedFile{disk: row.Disk, path: row.Path})
		}

		return nil
	})

	return found, files, err
}

/*
wipe deletes the messages' files' rows and empties the messages.
*/
func (a *PurgeSlackData) wipe(ctx context.Context, p *slackPurge, messages []ids.UUID) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), p.conn.WorkspaceID

		attachments, err := q.DeleteMessagesAttachments(ctx, models.DeleteMessagesAttachmentsParams{
			WorkspaceID: ws, MessageIds: messages,
		})
		if err != nil {
			return fmt.Errorf("delete attachments: %w", err)
		}

		wiped, err := q.WipeMessages(ctx, models.WipeMessagesParams{Now: a.Clock.Now(), WorkspaceID: ws, Ids: messages})
		if err != nil {
			return fmt.Errorf("wipe messages: %w", err)
		}

		p.counts["attachments"] += attachments
		p.counts["messages"] += wiped

		return nil
	})
}

/*
records deletes what the connection itself kept: deliveries, message
ids and triage posts a batch at a time, then teammate links, talks with
Operator, where broadcasts were posted, and the rooms.
*/
func (a *PurgeSlackData) records(ctx context.Context, p *slackPurge) error {
	ws, conn := p.conn.WorkspaceID, p.conn.ID

	batched := []struct {
		name   string
		delete func(q *models.Queries) (int64, error)
	}{
		{"channel_inbound_events", func(q *models.Queries) (int64, error) {
			return q.DeleteConnectionInboundEvents(ctx, models.DeleteConnectionInboundEventsParams{
				WorkspaceID: ws, ChannelConnectionID: &conn, RowLimit: slackRecordBatch,
			})
		}},
		{"message_refs", func(q *models.Queries) (int64, error) {
			return q.DeleteConnectionMessageRefs(ctx, models.DeleteConnectionMessageRefsParams{
				WorkspaceID: ws, ChannelConnectionID: conn, RowLimit: slackRecordBatch,
			})
		}},
		{"conversation_posts", func(q *models.Queries) (int64, error) {
			return q.DeleteConnectionConversationPosts(ctx, models.DeleteConnectionConversationPostsParams{
				WorkspaceID: ws, ChannelConnectionID: conn, RowLimit: slackRecordBatch,
			})
		}},
	}

	for _, step := range batched {
		if err := a.drain(ctx, p, step.name, step.delete); err != nil {
			return err
		}
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return a.rooms(ctx, models.New(tx), p)
	})
}

/*
slackRecordBatch is how many kept records one transaction deletes.
*/
const slackRecordBatch = 1000

/*
drain runs the delete, a batch a transaction, until nothing is left.
*/
func (a *PurgeSlackData) drain(
	ctx context.Context, p *slackPurge, name string, remove func(q *models.Queries) (int64, error),
) error {
	for {
		var deleted int64

		err := a.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
			var err error

			deleted, err = remove(models.New(tx))

			return err
		})
		if err != nil {
			return fmt.Errorf("delete %s: %w", name, err)
		}

		p.counts[name] += deleted

		if deleted < slackRecordBatch {
			return nil
		}
	}
}

/*
rooms deletes the connection's teammate links, talks with Operator and
rooms, forgetting first where broadcasts were posted in them.
*/
func (a *PurgeSlackData) rooms(ctx context.Context, q *models.Queries, p *slackPurge) error {
	ws, conn := p.conn.WorkspaceID, p.conn.ID

	links, err := q.DeleteConnectionMemberLinks(ctx, models.DeleteConnectionMemberLinksParams{
		WorkspaceID: ws, ChannelConnectionID: conn,
	})
	if err != nil {
		return fmt.Errorf("delete member links: %w", err)
	}

	threads, err := q.DeleteConnectionOperatorThreads(ctx, models.DeleteConnectionOperatorThreadsParams{
		WorkspaceID: ws, ChannelConnectionID: conn,
	})
	if err != nil {
		return fmt.Errorf("delete operator threads: %w", err)
	}

	if _, err := q.ScrubConnectionBroadcastRecipients(ctx, models.ScrubConnectionBroadcastRecipientsParams{
		Now: a.Clock.Now(), WorkspaceID: ws, ChannelConnectionID: conn,
	}); err != nil {
		return fmt.Errorf("scrub broadcast posts: %w", err)
	}

	rooms, err := q.DeleteConnectionRooms(ctx, models.DeleteConnectionRoomsParams{
		WorkspaceID: ws, ChannelConnectionID: conn,
	})
	if err != nil {
		return fmt.Errorf("delete rooms: %w", err)
	}

	p.counts["member_links"] += links
	p.counts["operator_threads"] += threads
	p.counts["rooms"] += rooms

	return nil
}
