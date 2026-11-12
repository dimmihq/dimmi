/*
|--------------------------------------------------------------------------
| Imported History
|--------------------------------------------------------------------------
|
| A workspace moving from another tool brings its old conversations
| along, with the times they really happened and who really wrote them.
| History is replayed, not lived again: the rows and the inbox's own
| columns are written as if each entry arrived at its time, but no event
| goes out, so no workflow, notification, assignment, agent or email
| reacts to something that is long over. Entries arrive oldest first, so
| the first and last message times and the customer's wait come out
| right. Only callers who may run imports bring history in.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
importSkew is how far ahead of the server's clock an imported time may
be, for a sender whose clock runs a little fast.
*/
const importSkew = time.Minute

/*
importFloor returns the oldest time history may carry.
*/
func importFloor() time.Time {
	return time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
}

/*
Backfill is how an entry of imported history is told apart: when it
happened, zero for now, the id it had in the other tool, and the member
who wrote it, nil for the caller. The member may have left the
workspace since; history keeps who really wrote it.
*/
type Backfill struct {
	At         time.Time
	ExternalID *string
	AuthorID   *ids.UUID
}

/*
CanImport refuses a caller who may not bring history in.
*/
func CanImport(m authz.Membership) error {
	if !m.Can("imports:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
CheckImportTime refuses a time ahead of now or before the year 2000,
blaming the field named.
*/
func CheckImportTime(at, now time.Time, field string) error {
	if at.After(now.Add(importSkew)) {
		return exceptions.ErrImportTimeInFuture.OnField(field)
	}

	if at.Before(importFloor()) {
		return exceptions.ErrImportTimeTooEarly.OnField(field)
	}

	return nil
}

/*
When returns the time of the entry, now when none was given.
*/
func (b Backfill) When(now time.Time) time.Time {
	if b.At.IsZero() {
		return now
	}

	return b.At
}

/*
Latest returns the time of the conversation's newest entry: its start,
its last message, its last activity or its close, whichever came last.
Imported history may not land before it.
*/
func Latest(c models.Conversation) time.Time {
	latest := c.CreatedAt

	for _, t := range []*time.Time{c.LastMessageAt, c.LastActivityAt, c.ClosedAt} {
		if t != nil && t.After(latest) {
			latest = *t
		}
	}

	return latest
}

/*
ImportMetadata returns what an imported entry keeps about where it came
from: that it was imported, and the id it had in the other tool.
*/
func ImportMetadata(externalID *string) (json.RawMessage, error) {
	data := map[string]any{"imported": true}
	if externalID != nil {
		data["external_id"] = *externalID
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode import metadata: %w", err)
	}

	return out, nil
}

/*
Importing returns a copy of the writer that replays history at the time:
it dispatches nothing and writes every row at that moment.
*/
func (w *Writer) Importing(at time.Time) *Writer {
	clone := *w
	clone.Imported = true
	clone.Now = at

	return &clone
}

/*
AsAuthor returns a writer acting as the member named, the writer's own
actor when none is, refusing an id that is no member of the workspace.
*/
func (w *Writer) AsAuthor(ctx context.Context, ws ids.UUID, author *ids.UUID) (*Writer, error) {
	if author == nil {
		return w, nil
	}

	members, err := Members(ctx, w.Q, ws, []ids.UUID{*author})
	if err != nil {
		return nil, fmt.Errorf("load author: %w", err)
	}

	member, ok := members[*author]
	if !ok {
		return nil, exceptions.ErrNotFound.OnField("author_id")
	}

	return w.AsMember(member.Member.ID, member.User.Name), nil
}

/*
ImportedEntry is one imported message or note: its type and body,
whether a teammate wrote it, and the key that keeps it from being
appended twice.
*/
type ImportedEntry struct {
	Kind     string
	Body     string
	ByMember bool
	Key      *string
}

/*
ImportedMessage returns the entry to append for an imported message or
note: a teammate's message counts as delivered long ago, and nothing
leaves the workspace for it.
*/
func ImportedMessage(e ImportedEntry, b Backfill) (NewMessage, error) {
	metadata, err := ImportMetadata(b.ExternalID)
	if err != nil {
		return NewMessage{}, err
	}

	m := NewMessage{Type: e.Kind, Body: e.Body, DedupeKey: e.Key, Metadata: metadata}
	if e.Kind == string(enums.MessageTypeMessage) && e.ByMember {
		m.DeliveryStatus = new(string(enums.DeliveryStatusDelivered))
	}

	return m, nil
}
