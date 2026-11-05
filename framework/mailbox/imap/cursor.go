/*
|--------------------------------------------------------------------------
| Reading Changes
|--------------------------------------------------------------------------
|
| The cursor keeps, per folder, its UIDVALIDITY and the last UID read. A
| new connection starts at the newest message, so connecting a mailbox
| never imports its history. When a server renumbers a folder, which
| UIDVALIDITY changing says, the folder is read again from a day before
| the last sync, and the application drops what it already holds.
|
*/

package imap

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
rescanMargin is how far before the last sync a renumbered folder is read
again.
*/
const rescanMargin = 24 * time.Hour

/*
folderCursor is the position in one folder.
*/
type folderCursor struct {
	UIDValidity uint32 `json:"uidvalidity"`
	LastUID     uint32 `json:"last_uid"`
}

/*
cursor is the position in every folder and when it was taken.
*/
type cursor struct {
	Received folderCursor `json:"received"`
	Sent     folderCursor `json:"sent"`
	At       time.Time    `json:"at"`
}

func (c *cursor) of(kind mailbox.Folder) *folderCursor {
	if kind == mailbox.FolderSent {
		return &c.Sent
	}

	return &c.Received
}

/*
Cursor returns a cursor at the newest message of each folder.
*/
func (s *Session) Cursor(ctx context.Context) ([]byte, error) {
	defer context.AfterFunc(ctx, func() { _ = s.client.Close() })()

	at := cursor{At: s.opts.Clock.Now()}

	for _, f := range s.folders() {
		data, err := s.examine(f.name)
		if err != nil {
			return nil, err
		}

		*at.of(f.kind) = folderCursor{UIDValidity: data.UIDValidity, LastUID: uint32(data.UIDNext) - 1}
	}

	return json.Marshal(at)
}

/*
Changes reads the messages after the cursor, at most a batch, Sent first.
A cursor it cannot read is reported as mailbox.ErrCursorExpired.
*/
func (s *Session) Changes(ctx context.Context, raw []byte) (Batch, error) {
	defer context.AfterFunc(ctx, func() { _ = s.client.Close() })()

	var at cursor
	if err := json.Unmarshal(raw, &at); err != nil {
		return Batch{}, fmt.Errorf("%w: %w", mailbox.ErrCursorExpired, err)
	}

	batch := Batch{}
	room := s.opts.BatchSize

	for _, f := range s.folders() {
		messages, more, err := s.read(f, at.of(f.kind), at.At, room)
		if err != nil {
			return Batch{}, err
		}

		batch.Messages = append(batch.Messages, messages...)
		room -= len(messages)

		if more {
			batch.More = true

			break
		}
	}

	at.At = s.opts.Clock.Now()

	next, err := json.Marshal(at)
	if err != nil {
		return Batch{}, fmt.Errorf("encode imap cursor: %w", err)
	}

	batch.Cursor = next

	return batch, nil
}

func (s *Session) examine(name string) (*imap.SelectData, error) {
	data, err := s.client.Select(name, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("%w: examine %s: %w", ErrUnreachable, name, err)
	}

	return data, nil
}

/*
read reads up to room new messages of one folder and moves its cursor
past them. A folder seen for the first time starts at its newest message.
*/
func (s *Session) read(f folder, at *folderCursor, synced time.Time, room int) ([]Message, bool, error) {
	data, err := s.examine(f.name)
	if err != nil {
		return nil, false, err
	}

	newest := uint32(data.UIDNext) - 1

	if at.UIDValidity == 0 {
		*at = folderCursor{UIDValidity: data.UIDValidity, LastUID: newest}

		return nil, false, nil
	}

	uids, err := s.pending(data, *at, synced)
	if err != nil {
		return nil, false, err
	}

	more := len(uids) > room
	uids = uids[:min(len(uids), room)]

	messages, err := s.fetch(f.kind, data.UIDValidity, uids)
	if err != nil {
		return nil, false, err
	}

	*at = folderCursor{UIDValidity: data.UIDValidity, LastUID: newest}
	if more {
		at.LastUID = uint32(uids[len(uids)-1])
	}

	return messages, more, nil
}

/*
pending lists the UIDs to read, oldest first: those after the cursor, or
after a renumbering, those since a day before the last sync.
*/
func (s *Session) pending(data *imap.SelectData, at folderCursor, synced time.Time) ([]imap.UID, error) {
	criteria := &imap.SearchCriteria{UID: []imap.UIDSet{{{Start: imap.UID(at.LastUID + 1), Stop: 0}}}}
	floor := at.LastUID

	if at.UIDValidity != data.UIDValidity {
		criteria = &imap.SearchCriteria{Since: synced.Add(-rescanMargin)}
		floor = 0
	}

	found, err := s.client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("%w: search: %w", ErrUnreachable, err)
	}

	uids := slices.DeleteFunc(found.AllUIDs(), func(uid imap.UID) bool { return uint32(uid) <= floor })
	slices.Sort(uids)

	return uids, nil
}
