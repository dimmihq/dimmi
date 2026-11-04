package conversations_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/drafts"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const draftsPath = "/api/v1/drafts"

/*
draft is the body of a draft save of the conversation and mode, written
at the moment.
*/
func draft(conversation, mode, body string, at time.Time) testkit.JSON {
	out := testkit.JSON{"mode": mode, "body": body, "client_updated_at": at}
	if conversation != "" {
		out["conversation_id"] = conversation
	}

	return out
}

/*
inTenant runs fn on the workspace's tables as the application role.
*/
func inTenant(t *testing.T, tc *tests.TestCase, fn func(ctx context.Context, q *models.Queries) error) {
	t.Helper()

	ctx := tenancy.With(t.Context(), tc.Workspace.Workspace.ID.String())
	require.NoError(t, tc.ApplicationRole().Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	}))
}

/*
stagedFile adds an upload no message uses yet, created at the moment.
*/
func stagedFile(t *testing.T, tc *tests.TestCase, name string) models.Attachment {
	t.Helper()

	var out models.Attachment

	inTenant(t, tc, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.CreateAttachment(ctx, models.CreateAttachmentParams{
			WorkspaceID: tc.Workspace.Workspace.ID, UploadedByType: "member", UploadedByID: &tc.Workspace.Owner.ID,
			Disk: "local", Path: "uploads/" + name, Filename: name, ContentType: "application/pdf",
			SizeBytes: 1024, ScanStatus: "clean", Now: tc.Clock.Now(),
		})

		return err
	})

	return out
}

/*
draftChange is what a draft.updated broadcast says.
*/
type draftChange struct {
	Conversation *string `json:"conversation_id"`
	Mode         string  `json:"mode"`
	ID           *string `json:"id"`
}

/*
nextChange reads the next draft.updated broadcast, skipping the others
the member's channel carries.
*/
func nextChange(t *testing.T, tab *testkit.Realtime) draftChange {
	t.Helper()

	for {
		if p := tab.Publication(); p.Event == "draft.updated" {
			return changeOf(t, p)
		}
	}
}

/*
changeOf reads a draft.updated broadcast.
*/
func changeOf(t *testing.T, p testkit.RealtimePublication) draftChange {
	t.Helper()

	var out draftChange
	require.NoError(t, json.Unmarshal(p.Data, &out))

	return out
}

func TestDrafts(t *testing.T) {
	t.Parallel()

	t.Run("keeps a draft per conversation and mode, and lists the member's own", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		now := d.tc.Clock.Now()

		reply := d.owner.PostJSON(draftsPath, draft(conv, "reply", "Refunding", now)).AssertCreated().
			AssertJSONPath("data.object", "draft").AssertJSONPath("data.mode", "reply").
			AssertJSONPath("data.conversation_id", conv).AssertJSONPath("data.recipients", map[string]any{})
		d.owner.PostJSON(draftsPath, draft(conv, "note", "Check Stripe", now)).AssertCreated()
		d.owner.PostJSON(draftsPath, draft("", "new", "Hello", now)).AssertCreated().
			AssertJSONPath("data.conversation_id", nil)

		d.owner.GetJSON(draftsPath).AssertOK().AssertJSONPath("data.#", 3)
		d.owner.GetJSON(draftsPath+"?filter[conversation]="+conv).AssertJSONPath("data.#", 2)
		d.owner.GetJSON(draftsPath+"?filter[mode]=note").AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.body", "Check Stripe")

		d.owner.PostJSON(draftsPath, draft(conv, "reply", "Again", now)).AssertStatus(409).
			AssertJSONPath("code", "draft_exists")
		d.owner.PostJSON(draftsPath+"?upsert=true", draft(conv, "reply", "Refunded", now.Add(time.Second))).
			AssertOK().AssertJSONPath("meta.upserted", true).AssertJSONPath("data.id", reply.String("data.id")).
			AssertJSONPath("data.body", "Refunded")
	})

	t.Run("keeps the newest write and answers a stale one with what is stored", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		phone := d.tc.Clock.Now().Add(time.Minute)

		id := d.owner.PostJSON(draftsPath, draft(conv, "reply", "From the phone", phone)).AssertCreated().
			String("data.id")

		old := testkit.JSON{"body": "Old tab", "client_updated_at": phone.Add(-time.Hour)}
		d.owner.PatchJSON(draftsPath+"/"+id, old).AssertOK().AssertJSONPath("meta.stale", true).
			AssertJSONPath("data.body", "From the phone")
		d.owner.PostJSON(draftsPath+"?upsert=true", draft(conv, "reply", "Old tab", phone.Add(-time.Hour))).
			AssertOK().AssertJSONPath("meta.stale", true).AssertJSONPath("meta.upserted", false)

		d.owner.PatchJSON(draftsPath+"/"+id, testkit.JSON{
			"body": "Newer", "client_updated_at": phone.Add(time.Second),
			"recipients": testkit.JSON{"cc": []string{"boss@example.com"}},
		}).AssertOK().AssertJSONPath("meta.stale", false).AssertJSONPath("data.body", "Newer").
			AssertJSONPath("data.recipients.cc", []any{"boss@example.com"})
	})

	t.Run("hides one member's drafts from every other, whatever their role", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		id := d.owner.PostJSON(draftsPath, draft(conv, "reply", "Mine", d.tc.Clock.Now())).AssertCreated().
			String("data.id")
		admin, _ := d.join(t, enums.RoleAdmin)

		admin.GetJSON(draftsPath).AssertOK().AssertJSONPath("data.#", 0)
		admin.PatchJSON(draftsPath+"/"+id, testkit.JSON{"body": "x", "client_updated_at": d.tc.Clock.Now()}).
			AssertNotFound()
		admin.Delete(draftsPath + "/" + id).AssertNotFound()
		admin.PostJSON(draftsPath, draft(conv, "reply", "Theirs", d.tc.Clock.Now())).AssertCreated()
		d.owner.Delete(draftsPath + "/nonsense").AssertNotFound()
	})

	t.Run("refuses targets, sizes and files a draft cannot hold", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		now := d.tc.Clock.Now()

		d.owner.PostJSON(draftsPath, draft("", "reply", "x", now)).AssertStatus(422).
			AssertJSONPath("code", "invalid_draft_target")
		d.owner.PostJSON(draftsPath, draft(conv, "new", "x", now)).AssertStatus(422)
		d.owner.PostJSON(draftsPath, draft(conv, "aside", "x", now)).AssertValidationError("mode")
		d.owner.PostJSON(draftsPath, draft("conv_nope", "reply", "x", now)).AssertNotFound()
		d.owner.PostJSON(draftsPath, draft(conv, "reply", strings.Repeat("x", 65*1024), now)).AssertStatus(422).
			AssertJSONPath("code", "draft_too_large")

		files := draft(conv, "reply", "x", now)
		files["attachment_ids"] = []string{models.PublicID(models.PrefixAttachment, ids.New())}
		d.owner.PostJSON(draftsPath, files).AssertStatus(422).AssertJSONPath("code", "invalid_draft_attachments")
		files["attachment_ids"] = []string{"att_nonsense"}
		d.owner.PostJSON(draftsPath, files).AssertStatus(422).AssertJSONPath("code", "invalid_draft_attachments")

		file := models.PublicID(models.PrefixAttachment, stagedFile(t, d.tc, "a.pdf").ID)
		files["attachment_ids"] = []string{file}
		d.owner.PostJSON(draftsPath, files).AssertCreated().AssertJSONPath("data.attachment_ids", []any{file})
	})

	t.Run("hides a draft whose conversation went to the trash, and keeps it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		id := d.owner.PostJSON(draftsPath, draft(conv, "reply", "Mine", d.tc.Clock.Now())).AssertCreated().
			String("data.id")

		_, err := d.tc.App.DB.Exec(t.Context(), "UPDATE conversations SET deleted_at = now()")
		require.NoError(t, err)

		d.owner.GetJSON(draftsPath).AssertJSONPath("data.#", 0)
		d.owner.GetJSON(draftsPath + "?filter[conversation]=" + conv).AssertNotFound()
		d.owner.GetJSON(draftsPath + "?filter[conversation]=nonsense").AssertNotFound()
		d.owner.PatchJSON(draftsPath+"/"+id, testkit.JSON{"body": "x", "client_updated_at": d.tc.Clock.Now()}).
			AssertNotFound()
		d.tc.Table("conversation_drafts").AssertCount(1, testkit.Row{"body": "Mine"})
	})

	t.Run("ends the matching draft when the reply, note or new conversation is sent", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		now := d.tc.Clock.Now()
		d.owner.PostJSON(draftsPath, draft(conv, "reply", "Refunding", now)).AssertCreated()
		d.owner.PostJSON(draftsPath, draft(conv, "note", "Check", now)).AssertCreated()
		d.owner.PostJSON(draftsPath, draft("", "new", "Hello", now)).AssertCreated()
		tab := d.tc.ListenAs(d.tc.Owner, d.tc.RealtimeURL())
		require.Nil(t, tab.Subscribe(appbroadcasting.MemberChannel(d.tc.Workspace.Owner.ID)).Error)

		d.owner.PostJSON("/api/v1/conversations/"+conv+"/replies", testkit.JSON{"body": "Refunded"}).AssertCreated()

		assert.Equal(t, draftChange{Conversation: &conv, Mode: "reply"}, nextChange(t, tab))
		d.owner.GetJSON(draftsPath+"?filter[conversation]="+conv).AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.mode", "note")

		d.owner.PostJSON("/api/v1/conversations/"+conv+"/notes", testkit.JSON{"body": "Done"}).AssertCreated()
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "member", "body": "Hello",
			"contact": testkit.JSON{"email": "bob@example.com", "name": "Bob"},
		}).AssertCreated()
		d.owner.GetJSON(draftsPath).AssertJSONPath("data.#", 0)

		d.owner.PostJSON("/api/v1/conversations/"+conv+"/replies", testkit.JSON{"body": "x", "draft_id": "drf_x"}).
			AssertStatus(422)
	})

	t.Run("tells the member's other tabs when a draft is saved or discarded", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		tab := d.tc.ListenAs(d.tc.Owner, d.tc.RealtimeURL())
		require.Nil(t, tab.Subscribe(appbroadcasting.MemberChannel(d.tc.Workspace.Owner.ID)).Error)

		id := d.owner.PostJSON(draftsPath, draft("", "new", "Hi", d.tc.Clock.Now())).AssertCreated().String("data.id")
		saved := tab.Publication()
		assert.Equal(t, "draft.updated", saved.Event)
		assert.Equal(t, draftChange{Mode: "new", ID: &id}, changeOf(t, saved))

		d.owner.Delete(draftsPath + "/" + id).AssertNoContent()
		gone := tab.Publication()
		assert.Equal(t, "draft.updated", gone.Event)
		assert.Equal(t, draftChange{Mode: "new"}, changeOf(t, gone))
	})
}

func TestDraftRetention(t *testing.T) {
	t.Parallel()

	t.Run("purges drafts untouched past the retention period, keeping their files until then", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		file := stagedFile(t, d.tc, "kept.pdf")
		stale := draft("", "new", "Old", d.tc.Clock.Now())
		stale["attachment_ids"] = []string{models.PublicID(models.PrefixAttachment, file.ID)}
		d.owner.PostJSON(draftsPath, stale).AssertCreated()
		d.tc.Clock.Travel(25 * time.Hour)

		var orphans []models.Attachment

		inTenant(t, d.tc, func(ctx context.Context, q *models.Queries) error {
			var err error
			orphans, err = q.ListUnattachedAttachments(ctx, models.ListUnattachedAttachmentsParams{
				WorkspaceID: d.tc.Workspace.Workspace.ID, Before: d.tc.Clock.Now().Add(-24 * time.Hour), RowLimit: 10,
			})

			return err
		})
		assert.Empty(t, orphans)

		d.tc.Clock.Travel(30 * 24 * time.Hour)
		d.owner = in(d.tc, d.tc.Owner)
		conv := d.customer(t, "ann", "Billing", "Charged twice")
		d.owner.PostJSON(draftsPath, draft(conv, "reply", "Fresh", d.tc.Clock.Now())).AssertCreated()

		keep := &jobs.PurgeStaleDrafts{DB: d.tc.App.DB, Clock: d.tc.Clock}
		require.NoError(t, keep.Handle(t.Context(), nil))
		d.tc.Table("conversation_drafts").AssertCount(1, testkit.Row{"body": "Old"})

		purge := &jobs.PurgeStaleDrafts{DB: d.tc.App.DB, Clock: d.tc.Clock, RetentionDays: 30}
		require.NoError(t, purge.Handle(t.Context(), nil))
		d.tc.Table("conversation_drafts").AssertCount(0, testkit.Row{"body": "Old"})
		d.tc.Table("conversation_drafts").AssertCount(1, testkit.Row{"body": "Fresh"})
	})

	t.Run("moves drafts to the conversation merged into, unless one of the same mode is there", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		source, target := d.customer(t, "ann", "One", "First"), d.customer(t, "ann", "Two", "Second")
		now := d.tc.Clock.Now()
		d.owner.PostJSON(draftsPath, draft(source, "reply", "Source reply", now)).AssertCreated()
		d.owner.PostJSON(draftsPath, draft(source, "note", "Source note", now)).AssertCreated()
		d.owner.PostJSON(draftsPath, draft(target, "reply", "Target reply", now)).AssertCreated()

		inTenant(t, d.tc, func(ctx context.Context, q *models.Queries) error {
			return drafts.MoveOnMerge(ctx, q, drafts.Merge{
				Workspace: d.tc.Workspace.Workspace.ID, Source: decode(t, d, models.PrefixConversation, source),
				Target: decode(t, d, models.PrefixConversation, target),
				Now:    d.tc.Clock.Now(),
			})
		})

		d.owner.GetJSON(draftsPath+"?filter[conversation]="+target).AssertJSONPath("data.#", 2)
		d.tc.Table("conversation_drafts").AssertCount(1, testkit.Row{"body": "Source reply"})
		d.owner.GetJSON(draftsPath+"?filter[conversation]="+source).AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.body", "Source reply")
	})
}
