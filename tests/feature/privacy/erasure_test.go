package privacy_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
recordingEraser is a part of the product that counts the contacts it
was asked to erase, failing while told to.
*/
type recordingEraser struct {
	mu     sync.Mutex
	seen   []ids.UUID
	failed bool
}

/*
Name names the eraser.
*/
func (e *recordingEraser) Name() string { return "recording" }

/*
Erase records the contact, or fails once when told to.
*/
func (e *recordingEraser) Erase(_ context.Context, _, contactID ids.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.failed {
		e.failed = false

		return errors.New("store unavailable")
	}

	e.seen = append(e.seen, contactID)

	return nil
}

/*
holdAll is a retention guard that holds every customer.
*/
type holdAll struct{}

/*
MayDestroy holds everything.
*/
func (holdAll) MayDestroy(context.Context, ids.UUID, contracts.RetentionSubject) (bool, error) {
	return false, nil
}

/*
customer starts a conversation from ann and returns its path and her id.
*/
func customer(t *testing.T, d tests.Desk) (string, string) {
	t.Helper()

	res := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": "My order", "body": "Card ending 1111, call me",
		"contact": testkit.JSON{"email": "ann@example.com", "name": "Ann"},
	}).AssertCreated()

	return "/api/v1/conversations/" + res.String("data.id"), res.String("data.contact.id")
}

/*
erase runs the erase job's work on the request now.
*/
func erase(t *testing.T, tc *tests.TestCase, erasure string) erasures.Outcome {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
	require.NoError(t, err)

	outcome, err := tc.App.Actions.Inbox.EraseContact.Handle(t.Context(), erasures.EraseArgs{
		WorkspaceID: tc.Workspace.Workspace.ID, ErasureID: id,
	})
	require.NoError(t, err)

	return outcome
}

func TestContactErasure(t *testing.T) {
	t.Parallel()

	t.Run("hides the customer at once and keeps their conversations out of lists and replies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path, contact := customer(t, d)

		res := d.Owner.Delete("/api/v1/contacts/"+contact+"?erase=true").AssertStatus(202).
			AssertJSONPath("data.object", "contact_erasure").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.contact_id", contact)
		due, err := time.Parse(time.RFC3339, res.String("data.scheduled_for"))
		require.NoError(t, err)
		assert.WithinDuration(t, d.TC.Clock.Now().Add(7*24*time.Hour), due, time.Second)

		d.Owner.GetJSON("/api/v1/contacts").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi"}).
			AssertStatus(422).AssertJSONPath("code", "contact_unreachable_on_channel")
		d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertNotFound()

		again := d.Customer(t, "ann", "Another question").String("data.contact.id")
		assert.NotEqual(t, contact, again)
		d.TC.Table("audit_logs").AssertCount(1, testkit.Row{"event": "contact_erasure.requested"})
		d.Owner.GetJSON("/api/v1/contact-erasures").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON("/api/v1/contact-erasures/" + res.String("data.id")).AssertOK().
			AssertJSONMissing("data.identities_stash")
	})

	t.Run("brings the customer back on cancel, except an address someone took meanwhile", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ann again", "email": "ann@example.com"}).
			AssertCreated()

		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertOK().
			AssertJSONPath("data.status", "canceled").
			AssertJSONPath("data.identities_conflicted.0", "email:ann@example.com")
		d.Owner.GetJSON("/api/v1/contacts/"+contact).AssertOK().AssertJSONPath("data.name", "Ann")
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).
			AssertStatus(409).AssertJSONPath("code", "erasure_closed")
		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "pending"}).
			AssertValidationError("status")
	})

	t.Run("gives the addresses back when no one took them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")

		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertOK().
			AssertJSONMissing("data.identities_conflicted")
		d.Owner.GetJSON("/api/v1/contacts/"+contact+"/identities").AssertOK().
			AssertJSONPath("data.0.value", "ann@example.com")
	})

	t.Run("wipes the customer when the grace period ends and runs every eraser", func(t *testing.T) {
		t.Parallel()
		eraser := &recordingEraser{failed: true}
		d := tests.NewDesk(t)
		d.TC.App.ContactErasers.Register(eraser)
		erased := tests.Record[appevents.ContactErased](d.TC)
		path, contact := customer(t, d)
		file := storeAttachment(t, d.TC, path)
		d.Owner.PostJSON("/api/v1/contacts/"+contact+"/exports", nil).AssertStatus(202)
		d.TC.Drain()

		var zipPath string
		require.NoError(t, d.TC.Operator().QueryRow(t.Context(), `SELECT path FROM contact_exports`).Scan(&zipPath))
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")

		assert.Equal(t, erasures.Waiting, erase(t, d.TC, erasure))
		d.TC.Clock.Travel(8 * 24 * time.Hour)

		id, err := d.TC.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
		require.NoError(t, err)
		_, err = d.TC.App.Actions.Inbox.EraseContact.Handle(t.Context(), erasures.EraseArgs{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ErasureID: id,
		})
		require.ErrorContains(t, err, "erase recording")
		d.Owner.GetJSON("/api/v1/contact-erasures/"+erasure).AssertOK().AssertJSONPath("data.status", "pending")

		assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))
		assert.Len(t, eraser.seen, 1)
		require.Len(t, erased.All(), 1)
		assert.Equal(t, int64(1), erased.All()[0].Counts["conversations"])

		d.Owner.GetJSON("/api/v1/contact-erasures/"+erasure).AssertOK().
			AssertJSONPath("data.status", "completed").
			AssertJSONPath("data.counts.conversations", 1)
		d.Owner.GetJSON(path).AssertOK().
			AssertJSONPath("data.subject", nil).
			AssertJSONPath("data.contact.name", "Deleted contact")
		d.Owner.GetJSON(path+"/messages").AssertOK().AssertJSONPath("data.0.body", nil)
		d.TC.Table("messages").AssertCount(0, testkit.Row{"body": "Card ending 1111, call me"})
		d.TC.Table("contacts").AssertCount(0, testkit.Row{"email": "ann@example.com"})
		d.TC.Table("attachments").AssertCount(0)
		d.TC.Table("audit_logs").AssertCount(1, testkit.Row{"event": "contact_erasure.completed"})
		exists, err := file.disk.Exists(t.Context(), file.path)
		require.NoError(t, err)
		assert.False(t, exists)
		d.TC.Table("contact_exports").AssertCount(0)
		zips, err := file.disk.Exists(t.Context(), zipPath)
		require.NoError(t, err)
		assert.False(t, zips)

		assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("keeps a held customer hidden and untouched", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = holdAll{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		_, contact := customer(t, d)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		tc.Clock.Travel(8 * 24 * time.Hour)

		assert.Equal(t, erasures.Held, erase(t, tc, erasure))
		assert.Equal(t, erasures.Held, erase(t, tc, erasure))
		d.Owner.GetJSON("/api/v1/contact-erasures/"+erasure).AssertOK().AssertJSONPath("data.status", "pending_hold")
		tc.Table("messages").AssertCount(1, testkit.Row{"body": "Card ending 1111, call me"})
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"event": "contact_erasure.held"})
	})

	t.Run("erases at once only after the password was confirmed", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		d.TC.Clock.Travel(time.Hour)

		d.Owner.Delete("/api/v1/contacts/"+contact+"?erase=true&immediate=true").
			AssertStatus(423).AssertJSONPath("code", "password_confirmation_required")
		d.Owner.PostJSON("/api/v1/user/confirmation", testkit.JSON{
			"method": "password", "password": factories.Password,
		}).AssertCreated()
		res := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true&immediate=true").AssertStatus(202)
		due, err := time.Parse(time.RFC3339, res.String("data.scheduled_for"))
		require.NoError(t, err)
		assert.True(t, due.Equal(d.TC.Clock.Now()))

		d.TC.Drain()
		d.Owner.GetJSON("/api/v1/contact-erasures/"+res.String("data.id")).AssertOK().
			AssertJSONPath("data.status", "completed")
	})

	t.Run("lets only those who may erase customers ask", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		member, _ := d.Join(t, enums.RoleMember)

		member.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertForbidden()
		member.GetJSON("/api/v1/contact-erasures").AssertForbidden()
		d.Owner.Delete("/api/v1/contacts/ctc_bogus?erase=true").AssertNotFound()
		d.Owner.GetJSON("/api/v1/contact-erasures/ces_bogus").AssertNotFound()
		d.Owner.PatchJSON("/api/v1/contact-erasures/ces_bogus", testkit.JSON{"status": "canceled"}).AssertNotFound()
	})
}

/*
storedFile is a file the test put on a disk.
*/
type storedFile struct {
	disk filesystem.Filesystem
	path string
}

/*
storeAttachment puts a file on the default disk and attaches it to the
first message of the conversation.
*/
func storeAttachment(t *testing.T, tc *tests.TestCase, path string) storedFile {
	t.Helper()

	name := tc.App.Application.Config.Filesystems.Default
	disk, err := tc.App.Storage.Disk(name)
	require.NoError(t, err)

	key := "workspaces/" + tc.Workspace.Workspace.ID.String() + "/erasure-test/" + ids.New().String() + ".txt"
	require.NoError(t, disk.Put(t.Context(), key, []byte("receipt"), filesystem.PutOptions{}))

	conversation, err := tc.App.IDs.Decode(string(models.PrefixConversation), path[len("/api/v1/conversations/"):])
	require.NoError(t, err)

	_, err = tc.Operator().Exec(t.Context(), `
		INSERT INTO attachments
			(workspace_id, message_id, uploaded_by_type, disk, path, filename, content_type, size_bytes)
		SELECT workspace_id, id, 'contact', $2, $3, 'receipt.txt', 'text/plain', 7 FROM messages
		WHERE conversation_id = $1 ORDER BY created_at LIMIT 1`, conversation, name, key)
	require.NoError(t, err)

	return storedFile{disk: disk, path: key}
}
