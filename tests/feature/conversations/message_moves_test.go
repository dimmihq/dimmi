package conversations_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
eraser records the messages it was asked to erase, failing while fails
is above zero.
*/
type eraser struct {
	mu     sync.Mutex
	erased []ids.UUID
	fails  int
}

func (e *eraser) EraseMessage(_ context.Context, _, message ids.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.fails > 0 {
		e.fails--

		return errors.New("raw mail store is down")
	}

	e.erased = append(e.erased, message)

	return nil
}

func (e *eraser) seen() []ids.UUID {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]ids.UUID{}, e.erased...)
}

/*
entries returns the conversation's messages and notes, oldest first.
*/
func entries(d desk, path string) *testkit.Response {
	return d.owner.GetJSON(path + "/messages?filter[type]=message,note").AssertOK()
}

func TestMessageMoves(t *testing.T) {
	t.Parallel()

	t.Run("deletes a note to a tombstone, erases its files and copies, and tells the screens", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		hook := &eraser{}
		d.tc.App.MessageErasers.Register(hook)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Card", "My card is 4242")
		note := d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Card 4242 4242"}).AssertCreated().
			String("data.id")
		file := stagedFile(t, d.tc, "card.png")
		inTenant(t, d.tc, func(ctx context.Context, q *models.Queries) error {
			_, err := q.LinkAttachments(ctx, models.LinkAttachmentsParams{
				MessageID: new(decode(t, d, models.PrefixMessage, note)), Now: d.tc.Clock.Now(),
				WorkspaceID: d.tc.Workspace.Workspace.ID, Ids: []ids.UUID{file.ID},
			})

			return err
		})

		d.owner.Send("DELETE", path+"/messages/"+note, nil).AssertOK().AssertJSONPath("data.undone", false)
		d.tc.Drain()

		entries(d, path).AssertJSONPath("data.1.body", nil).AssertJSONPath("data.1.deleted_by.name", d.tc.Owner.Name)
		assert.Equal(t, []ids.UUID{decode(t, d, models.PrefixMessage, note)}, hook.seen())
		d.tc.Table("attachments").AssertCount(0, testkit.Row{"id": file.ID})
		d.owner.Send("DELETE", path+"/messages/"+note, nil).AssertNotFound()
	})

	t.Run("tries erasing again until every copy is gone", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		hook := &eraser{fails: 1}
		erase := &jobs.EraseMessageCopies{
			DB: d.tc.App.DB, Storage: d.tc.App.Storage, Erasers: &jobs.MessageErasers{},
		}
		erase.Erasers.Register(hook)
		job := &queue.Job[jobs.EraseMessageCopiesArgs]{Args: jobs.EraseMessageCopiesArgs{
			WorkspaceID: d.tc.Workspace.Workspace.ID, MessageID: ids.New(),
		}}

		require.Error(t, erase.Handle(t.Context(), job))
		require.NoError(t, erase.Handle(t.Context(), job))
		assert.Len(t, hook.seen(), 1)
	})

	t.Run("lets the writer delete their own, a moderator anyone's, and nobody an event", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, _ := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Card", "Hello")
		reply := d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi Ann"}).AssertCreated().String("data.id")
		note := member.PostJSON(path+"/notes", testkit.JSON{"body": "Mine"}).AssertCreated().String("data.id")
		customer := entries(d, path).String("data.0.id")
		event := d.owner.GetJSON(path + "/messages?filter[type]=event").String("data.0.id")

		member.Send("DELETE", path+"/messages/"+reply, nil).AssertForbidden()
		member.Send("DELETE", path+"/messages/"+customer, nil).AssertForbidden()
		member.Send("DELETE", path+"/messages/"+note, nil).AssertOK()
		d.owner.Send("DELETE", path+"/messages/"+event, nil).AssertStatus(422).
			AssertJSONPath("code", "message_not_deletable")

		d.owner.Send("DELETE", path+"/messages/"+reply, nil).AssertOK().
			AssertJSONPath("meta.conversation.last_message.id", customer)
		d.owner.Send("DELETE", path+"/messages/"+customer, nil).AssertOK()
		d.owner.Send("DELETE", path+"/messages/msg_nonsense", nil).AssertNotFound()
	})

	t.Run("edits the writer's own note and nothing else", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, _ := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Card", "Hello")
		note := d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Draft"}).AssertCreated().String("data.id")
		reply := d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi"}).AssertCreated().String("data.id")

		d.owner.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "Fixed"}).AssertOK().
			AssertJSONPath("data.body", "Fixed").AssertJSONPath("data.id", note)
		member.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "Mine now"}).AssertStatus(422).
			AssertJSONPath("code", "message_not_editable")
		d.owner.PatchJSON(path+"/messages/"+reply, testkit.JSON{"body": "x"}).AssertStatus(422)
		d.owner.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": ""}).AssertValidationError("body")
		d.owner.PatchJSON(path+"/messages/msg_nonsense", testkit.JSON{"body": "x"}).AssertNotFound()

		d.owner.Send("DELETE", path+"/messages/"+note, nil).AssertOK()
		d.owner.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "Back"}).AssertNotFound()
		assert.NotEmpty(t, entries(d, path).String("data.1.edited_at"))
	})
}
