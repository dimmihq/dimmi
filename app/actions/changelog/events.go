/*
|--------------------------------------------------------------------------
| Changelog Events
|--------------------------------------------------------------------------
|
| Going live, going live again with edits and being taken back each
| dispatch an event in the transaction that changed what readers see,
| so webhooks hear about the entry. Webhooks read the entry back the
| way the API shows it.
|
*/

package changelog

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Load reads one entry as the API shows it, with its labels and authors.
A missing entry answers database.ErrNotFound.
*/
func Load(ctx context.Context, q *models.Queries, ws, id ids.UUID) (Entry, error) {
	row, err := q.FindChangelogEntry(ctx, models.FindChangelogEntryParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return Entry{}, err
	}

	return dress(ctx, q, ws, row)
}

/*
announce dispatches the event in the transaction, when events are
wired.
*/
func (e *Entries) announce(ctx context.Context, tx *database.Tx, ev appevents.Named) error {
	if e.Events == nil {
		return nil
	}

	if err := e.Events.Dispatch(ctx, tx, ev); err != nil {
		return fmt.Errorf("dispatch %s: %w", ev.EventName(), err)
	}

	return nil
}

/*
wentLive announces an entry published, or updated when it was already
live. A nil member means its schedule published it.
*/
func (e *Entries) wentLive(ctx context.Context, tx *database.Tx, before models.ChangelogEntry, by *ids.UUID) error {
	base := e.base(before.WorkspaceID, by)
	if before.Status == "published" {
		return e.announce(ctx, tx, appevents.ChangelogEntryUpdated{Base: base, EntryID: before.ID})
	}

	return e.announce(ctx, tx, appevents.ChangelogEntryPublished{Base: base, EntryID: before.ID})
}

/*
base is the header of an entry's event, caused by the member or the
system when there is none.
*/
func (e *Entries) base(ws ids.UUID, by *ids.UUID) appevents.Base {
	actor := appevents.Actor{Type: "system", ID: ids.Nil()}
	if by != nil {
		actor = appevents.Actor{Type: "member", ID: *by}
	}

	return appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: e.Clock.Now(), Actor: actor}
}
