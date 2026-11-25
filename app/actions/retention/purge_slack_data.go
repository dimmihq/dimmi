/*
|--------------------------------------------------------------------------
| Purge Slack Data After An Uninstall
|--------------------------------------------------------------------------
|
| Slack lets an app keep what it read only while it is installed. Once a
| workspace's Slack connection ended, because the app was uninstalled,
| its tokens were revoked or a teammate disconnected it, the connection
| waits out a grace period, so connecting again loses nothing, and then
| everything Slack brought in is deleted.
|
| The conversations stay as empty shells, as an erased customer's do, so
| reports and counts do not change: every message loses its text, files
| and place in Slack, and the conversation its subject, source and
| customer. Contacts known only from Slack are erased; others lose their
| Slack identity. The kept deliveries, rooms, teammate links, message
| ids, triage posts and talks with Operator in Slack are deleted, and the
| connection forgets its team. The purge starts by retiring the
| connection, so Slack connected again later starts fresh. Every step
| finds nothing to do when it already ran, and the audit log records the
| counts once all of it is gone.
|
*/

package retention

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactErasers removes what the parts of the product kept about a
contact about to be deleted.
*/
type ContactErasers interface {
	/*
		Run runs every eraser on the contact.
	*/
	Run(ctx context.Context, workspaceID, contactID ids.UUID) error
}

/*
AuditWriter writes an entry to a workspace's audit log.
*/
type AuditWriter interface {
	/*
		Handle writes the entry.
	*/
	Handle(ctx context.Context, e audit.Entry) (models.AuditLog, error)
}

/*
PurgeSlackData deletes what ended Slack connections brought in. Grace
is how long after its end a connection keeps its data.
*/
type PurgeSlackData struct {
	DB        *database.DB
	Clock     clock.Clock
	Storage   Disks
	Retention contracts.RetentionGuard
	Erasers   ContactErasers
	Audit     AuditWriter
	Grace     time.Duration
}

/*
slackPurge is one connection's purge under way: what it removed so far
and whether a retention hold kept something back.
*/
type slackPurge struct {
	conn   models.ChannelConnection
	counts map[string]int64
	held   bool
}

/*
slackContacts are the customers of a batch of conversations and which
of them exist only because of the connection.
*/
type slackContacts struct {
	all  []ids.UUID
	only []ids.UUID
}

/*
Handle purges every connection whose grace period is over, in every
live workspace, and returns how many it finished.
*/
func (a *PurgeSlackData) Handle(ctx context.Context) (int, error) {
	workspaces, err := models.New(a.DB).ListWorkspaces(ctx)
	if err != nil {
		return 0, fmt.Errorf("list workspaces: %w", err)
	}

	var (
		finished int
		errs     []error
	)

	for _, w := range workspaces {
		done, err := a.Workspace(ctx, w.ID)
		finished += done

		if err != nil {
			errs = append(errs, fmt.Errorf("workspace %s: %w", w.ID, err))
		}
	}

	return finished, errors.Join(errs...)
}

/*
Workspace purges the workspace's connections whose grace period is
over and returns how many it finished.
*/
func (a *PurgeSlackData) Workspace(ctx context.Context, workspaceID ids.UUID) (int, error) {
	ctx = tenancy.With(ctx, workspaceID.String())
	cutoff := a.Clock.Now().Add(-a.Grace)

	var due []models.ChannelConnection

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		due, err = models.New(tx).ListEndedSlackConnections(ctx, models.ListEndedSlackConnectionsParams{
			WorkspaceID: workspaceID, Cutoff: cutoff,
		})

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("list ended slack connections: %w", err)
	}

	finished := 0

	for _, conn := range due {
		done, err := a.connection(ctx, conn, cutoff)
		if err != nil {
			return finished, fmt.Errorf("purge slack connection %s: %w", conn.ID, err)
		}

		if done {
			finished++
		}
	}

	return finished, nil
}

/*
connection purges one connection, reporting false when it was connected
again meanwhile or a retention hold kept part of it for a later round.
*/
func (a *PurgeSlackData) connection(
	ctx context.Context, conn models.ChannelConnection, cutoff time.Time,
) (bool, error) {
	claimed, err := a.claim(ctx, conn, cutoff)
	if err != nil || !claimed {
		return false, err
	}

	p := &slackPurge{conn: conn, counts: map[string]int64{}}

	steps := []func(context.Context, *slackPurge) error{a.conversations, a.strayMessages, a.records}
	for _, step := range steps {
		if err := step(ctx, p); err != nil {
			return false, err
		}
	}

	if p.held {
		return false, nil
	}

	return true, a.finish(ctx, p)
}

/*
claim retires the connection as its purge starts, reporting false when
it is no longer due: connected again, or purged by another run.
*/
func (a *PurgeSlackData) claim(ctx context.Context, conn models.ChannelConnection, cutoff time.Time) (bool, error) {
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).ClaimEndedSlackConnection(ctx, models.ClaimEndedSlackConnectionParams{
			Now: a.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID, Cutoff: cutoff,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("claim connection: %w", err)
	}

	return true, nil
}

/*
conversations empties the connection's conversations, a batch at a
time. A batch holding a conversation the retention guard keeps is left
whole for a later round.
*/
func (a *PurgeSlackData) conversations(ctx context.Context, p *slackPurge) error {
	after := ids.Nil()

	for {
		var found []ids.UUID

		err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error

			found, err = models.New(tx).ListConnectionConversations(ctx, models.ListConnectionConversationsParams{
				WorkspaceID: p.conn.WorkspaceID, ChannelConnectionID: p.conn.ID, After: after, RowLimit: purgeBatch,
			})

			return err
		})
		if err != nil {
			return fmt.Errorf("list conversations: %w", err)
		}

		if len(found) == 0 {
			return nil
		}

		after = found[len(found)-1]

		if err := a.batch(ctx, p, found); err != nil {
			return err
		}
	}
}

/*
batch empties one batch of conversations unless the guard keeps one.
*/
func (a *PurgeSlackData) batch(ctx context.Context, p *slackPurge, conversations []ids.UUID) error {
	for _, id := range conversations {
		ok, err := a.Retention.MayDestroy(ctx, p.conn.WorkspaceID, contracts.RetentionSubject{
			Kind: contracts.RetentionConversation, ID: id,
		})
		if err != nil {
			return fmt.Errorf("ask retention guard: %w", err)
		}

		if !ok {
			p.held = true

			return nil
		}
	}

	contacts, only, err := a.prepare(ctx, p, conversations)
	if err != nil {
		return err
	}

	for _, contact := range only {
		if err := a.Erasers.Run(ctx, p.conn.WorkspaceID, contact); err != nil {
			return fmt.Errorf("erase contact traces: %w", err)
		}
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if err := a.empty(ctx, q, p, conversations); err != nil {
			return err
		}

		return a.forget(ctx, q, p, slackContacts{all: contacts, only: only})
	})
}

/*
prepare deletes the stored files of the conversations' messages and
returns the conversations' contacts and which of them exist only
because of the connection. Files go first: a run that stops before the
rows go finds them again, and a file gone already is no error.
*/
func (a *PurgeSlackData) prepare(ctx context.Context, p *slackPurge, conversations []ids.UUID,
) ([]ids.UUID, []ids.UUID, error) {
	var (
		files          []storedFile
		contacts, only []ids.UUID
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), p.conn.WorkspaceID

		rows, err := q.ListSlackConversationFiles(ctx, models.ListSlackConversationFilesParams{
			WorkspaceID: ws, Ids: conversations,
		})
		if err != nil {
			return fmt.Errorf("list attachments: %w", err)
		}

		for _, row := range rows {
			files = append(files, storedFile{disk: row.Disk, path: row.Path})
		}

		contacts, err = q.ListConversationContacts(ctx, models.ListConversationContactsParams{
			WorkspaceID: ws, Ids: conversations,
		})
		if err != nil {
			return fmt.Errorf("list contacts: %w", err)
		}

		only, err = q.ListSlackOnlyContacts(ctx, models.ListSlackOnlyContactsParams{
			WorkspaceID: ws, Ids: contacts, ChannelConnectionID: p.conn.ID,
		})
		if err != nil {
			return fmt.Errorf("list slack contacts: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return contacts, only, deleteStoredFiles(ctx, a.Storage, files)
}

/*
empty deletes the files' rows and the reactions of the conversations'
messages, empties the messages and hands the conversations, cleared, to
the placeholder.
*/
func (a *PurgeSlackData) empty(ctx context.Context, q *models.Queries, p *slackPurge, conversations []ids.UUID) error {
	ws, now := p.conn.WorkspaceID, a.Clock.Now()

	attachments, err := q.DeleteConversationAttachments(ctx, models.DeleteConversationAttachmentsParams{
		WorkspaceID: ws, Ids: conversations,
	})
	if err != nil {
		return fmt.Errorf("delete attachments: %w", err)
	}

	if _, err := q.DeleteConversationReactions(ctx, models.DeleteConversationReactionsParams{
		WorkspaceID: ws, Ids: conversations,
	}); err != nil {
		return fmt.Errorf("delete reactions: %w", err)
	}

	messages, err := q.WipeConversationMessages(ctx, models.WipeConversationMessagesParams{
		Now: now, WorkspaceID: ws, Ids: conversations,
	})
	if err != nil {
		return fmt.Errorf("wipe messages: %w", err)
	}

	placeholder, err := q.CreatePlaceholderContact(ctx, models.CreatePlaceholderContactParams{
		WorkspaceID: ws, Now: &now,
	})
	if err != nil {
		return fmt.Errorf("find placeholder: %w", err)
	}

	emptied, err := q.EmptyPurgedConversations(ctx, models.EmptyPurgedConversationsParams{
		PlaceholderID: placeholder.ID, Now: now, WorkspaceID: ws, Ids: conversations,
	})
	if err != nil {
		return fmt.Errorf("empty conversations: %w", err)
	}

	p.counts["attachments"] += attachments
	p.counts["messages"] += messages
	p.counts["conversations"] += emptied

	return nil
}

/*
forget deletes the contacts that exist only because of the connection,
their other conversations on it handed to the placeholder first, and
the Slack identities of the rest.
*/
func (a *PurgeSlackData) forget(ctx context.Context, q *models.Queries, p *slackPurge, who slackContacts) error {
	ws, now := p.conn.WorkspaceID, a.Clock.Now()
	contacts, only := who.all, who.only

	if len(only) > 0 {
		placeholder, err := q.CreatePlaceholderContact(ctx, models.CreatePlaceholderContactParams{
			WorkspaceID: ws, Now: &now,
		})
		if err != nil {
			return fmt.Errorf("find placeholder: %w", err)
		}

		if _, err := q.HandOverContactsConversations(ctx, models.HandOverContactsConversationsParams{
			PlaceholderID: placeholder.ID, Now: now, WorkspaceID: ws, ContactIds: only,
		}); err != nil {
			return fmt.Errorf("hand over conversations: %w", err)
		}

		deleted, err := q.DeleteSlackOnlyContacts(ctx, models.DeleteSlackOnlyContactsParams{WorkspaceID: ws, Ids: only})
		if err != nil {
			return fmt.Errorf("delete contacts: %w", err)
		}

		p.counts["contacts"] += deleted
	}

	identities, err := q.DeleteContactsSlackIdentities(ctx, models.DeleteContactsSlackIdentitiesParams{
		WorkspaceID: ws, ContactIds: contacts, ChannelConnectionID: p.conn.ID,
	})
	if err != nil {
		return fmt.Errorf("delete slack identities: %w", err)
	}

	p.counts["contact_identities"] += identities

	return nil
}

/*
finish makes the connection forget its Slack team and records the purge
in the audit log, with how much went and nothing of what it was.
*/
func (a *PurgeSlackData) finish(ctx context.Context, p *slackPurge) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).FinishSlackConnectionPurge(ctx, models.FinishSlackConnectionPurgeParams{
			Now: a.Clock.Now(), WorkspaceID: p.conn.WorkspaceID, ID: p.conn.ID,
		})
		if err != nil {
			return fmt.Errorf("finish purge: %w", err)
		}

		changes := map[string]any{"reason": "uninstalled"}
		if p.conn.EndedAt != nil {
			changes["ended_at"] = p.conn.EndedAt.UTC().Format(time.RFC3339)
		}

		for name, count := range p.counts {
			changes[name] = count
		}

		_, err = a.Audit.Handle(ctx, audit.Entry{
			WorkspaceID: p.conn.WorkspaceID, Event: audit.EventSlackDataDeleted, ActorID: ids.Nil(), Changes: changes,
			Subject: audit.Subject{Type: "channel_connection", ID: p.conn.ID, Label: "Slack"},
		})

		return err
	})
}

/*
deleteStoredFiles deletes the files from their disks, a file gone
already included.
*/
func deleteStoredFiles(ctx context.Context, storage Disks, files []storedFile) error {
	for _, file := range files {
		disk, err := storage.Disk(file.disk)
		if err != nil {
			return fmt.Errorf("open disk %s: %w", file.disk, err)
		}

		if err := disk.Delete(ctx, file.path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
			return fmt.Errorf("delete file: %w", err)
		}
	}

	return nil
}
