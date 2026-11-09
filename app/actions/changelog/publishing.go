/*
|--------------------------------------------------------------------------
| Publishing Entries
|--------------------------------------------------------------------------
|
| Publishing copies the working copy over what readers see and moves the
| help center's cache on, so every page shows it at once. An entry
| publishes now, on a date in the past for changes that shipped
| earlier, or later on its own when scheduled. Asking to tell
| subscribers queues the entry's one mailing; it is sent at most once,
| however often the entry is published again.
|
*/

package changelog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of publishing.
*/
var (
	ErrNothingToPublish = fault.New(fault.Conflict, "nothing_to_publish",
		"This entry has no changes to publish.")
	ErrScheduleInPast = fault.New(fault.Invalid, "schedule_in_past",
		"Pick a time in the future to schedule this entry.").OnField("scheduled_for")
	ErrAlreadyPublished = fault.New(fault.Conflict, "already_published",
		"This entry is already published. Unpublish it to schedule it.")
)

/*
PublishInput says how to publish: Notify mails active subscribers once,
At dates the entry, and ScheduledFor holds it until then instead.
*/
type PublishInput struct {
	Notify       bool
	At           *time.Time
	ScheduledFor *time.Time
}

/*
PublishEntryArgs names an entry whose time to publish has come.
*/
type PublishEntryArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	EntryID     ids.UUID `json:"entry_id"`
}

/*
Kind names the job.
*/
func (PublishEntryArgs) Kind() string { return "publish_changelog_entry" }

/*
MailEntryArgs names an entry to mail to its subscribers.
*/
type MailEntryArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	EntryID     ids.UUID `json:"entry_id"`
}

/*
Kind names the job.
*/
func (MailEntryArgs) Kind() string { return "mail_changelog_entry" }

/*
Publish makes the working copy live now, or schedules it.
*/
func (e *Entries) Publish(ctx context.Context, m authz.Membership, id ids.UUID, in PublishInput) (Entry, error) {
	if !m.Can("changelog:write") {
		return Entry{}, exceptions.ErrForbidden
	}
	if in.ScheduledFor != nil {
		return e.schedule(ctx, m, id, in)
	}
	ws := m.Workspace.ID
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockEntry(ctx, q, ws, id)
		if err != nil {
			return err
		}
		if current.Status == "published" && !current.HasUnpublishedChanges && in.At == nil && !in.Notify {
			return ErrNothingToPublish
		}
		row, err := e.goLive(ctx, tx, current, in)
		if err != nil {
			return err
		}
		out, err = dress(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
goLive renders and copies the working copy live inside the
transaction, bumps the cache and queues the mailing when one is due.
*/
func (e *Entries) goLive(ctx context.Context, tx *database.Tx, current models.ChangelogEntry, in PublishInput,
) (models.ChangelogEntry, error) {
	q, now := models.New(tx), e.Clock.Now()
	html, err := e.Renderer.HTML([]byte(current.BodyMarkdown))
	if err != nil {
		return current, fmt.Errorf("render changelog entry: %w", err)
	}
	at := in.At
	if current.Status == "published" && at == nil {
		at = current.PublishedAt
	}
	row, err := q.PublishChangelogEntry(ctx, models.PublishChangelogEntryParams{
		PublishedAt: at, Now: now, BodyHtml: &html.HTML, RendererVersion: RendererVersion, Notify: in.Notify,
		WorkspaceID: current.WorkspaceID, ID: current.ID,
	})
	if err != nil {
		return row, fmt.Errorf("publish changelog entry: %w", err)
	}
	if err := touch(ctx, q, row, now); err != nil {
		return row, err
	}
	if row.NotifySubscribers && row.NotifiedAt == nil {
		args := MailEntryArgs{WorkspaceID: row.WorkspaceID, EntryID: row.ID}
		if err := e.Queue.PushTx(ctx, tx, args); err != nil {
			return row, fmt.Errorf("queue changelog mailing: %w", err)
		}
	}
	return row, nil
}

/*
schedule holds a draft until its time, when a job publishes it.
*/
func (e *Entries) schedule(ctx context.Context, m authz.Membership, id ids.UUID, in PublishInput) (Entry, error) {
	ws, now := m.Workspace.ID, e.Clock.Now()
	if !in.ScheduledFor.After(now) {
		return Entry{}, ErrScheduleInPast
	}
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockEntry(ctx, q, ws, id)
		if err != nil {
			return err
		}
		if current.Status == "published" {
			return ErrAlreadyPublished
		}
		row, err := q.ScheduleChangelogEntry(ctx, models.ScheduleChangelogEntryParams{
			ScheduledFor: in.ScheduledFor, Notify: in.Notify, Now: now, WorkspaceID: ws, ID: id,
		})
		if err != nil {
			return fmt.Errorf("schedule changelog entry: %w", err)
		}
		args := PublishEntryArgs{WorkspaceID: ws, EntryID: id}
		if err := e.Queue.PushTx(ctx, tx, args, queue.At(*in.ScheduledFor)); err != nil {
			return fmt.Errorf("queue scheduled publish: %w", err)
		}
		out, err = dress(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
PublishDue publishes the entry when it is still scheduled and its time
has come. A job left over from an earlier schedule finds nothing to do.
*/
func (e *Entries) PublishDue(ctx context.Context, args PublishEntryArgs) error {
	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockEntry(ctx, q, args.WorkspaceID, args.EntryID)
		if errors.Is(err, ErrEntryNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		due := current.ScheduledFor != nil && !current.ScheduledFor.After(e.Clock.Now())
		if current.Status != "scheduled" || !due {
			return nil
		}
		_, err = e.goLive(ctx, tx, current, PublishInput{Notify: current.NotifySubscribers, At: current.ScheduledFor})
		return err
	})
}

/*
Unpublish takes an entry back to a draft: off the site, or no longer
scheduled.
*/
func (e *Entries) Unpublish(ctx context.Context, m authz.Membership, id ids.UUID) (Entry, error) {
	if !m.Can("changelog:write") {
		return Entry{}, exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, e.Clock.Now()
	var out Entry
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := lockEntry(ctx, q, ws, id); err != nil {
			return err
		}
		row, err := q.UnpublishChangelogEntry(ctx, models.UnpublishChangelogEntryParams{
			Now: now, WorkspaceID: ws, ID: id,
		})
		if err != nil {
			return fmt.Errorf("unpublish changelog entry: %w", err)
		}
		if err := touch(ctx, q, row, now); err != nil {
			return err
		}
		out, err = dress(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
Delete removes an entry from the site and the team's list.
*/
func (e *Entries) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("changelog:write") {
		return exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, e.Clock.Now()
	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockEntry(ctx, q, ws, id)
		if err != nil {
			return err
		}
		if err := q.DeleteChangelogEntry(ctx, models.DeleteChangelogEntryParams{
			Now: &now, WorkspaceID: ws, ID: id,
		}); err != nil {
			return fmt.Errorf("delete changelog entry: %w", err)
		}
		return touch(ctx, q, current, now)
	})
}

/*
touch moves the help center's cache on, so readers see the change.
*/
func touch(ctx context.Context, q *models.Queries, row models.ChangelogEntry, now time.Time) error {
	if err := q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
		WorkspaceID: row.WorkspaceID, ID: row.HelpCenterID, Now: now,
	}); err != nil {
		return fmt.Errorf("touch help center cache: %w", err)
	}
	return nil
}
