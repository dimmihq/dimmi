package tags_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestTagVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("adds, renames, recolors and removes a tag", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		created := tests.Record[appevents.TagCreated](d.TC)
		updated := tests.Record[appevents.TagUpdated](d.TC)
		deleted := tests.Record[appevents.TagDeleted](d.TC)

		res := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": " Billing ", "color": "#f59e0b"}).AssertCreated().
			AssertJSONPath("data.object", "tag").
			AssertJSONPath("data.name", "Billing").
			AssertJSONPath("data.color", "#f59e0b")
		path := "/api/v1/tags/" + res.String("data.id")

		d.Owner.PatchJSON(path, testkit.JSON{"name": "Invoices", "color": nil}).AssertOK().
			AssertJSONPath("data.name", "Invoices").
			AssertJSONPath("data.color", nil)
		d.Owner.PatchJSON(path, testkit.JSON{"name": "Invoices"}).AssertOK()
		require.Len(t, updated.All(), 1)
		assert.Equal(t, []string{"name", "color"}, updated.All()[0].Changes)

		d.Owner.GetJSON("/api/v1/tags").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.Delete(path).AssertNoContent()
		d.Owner.Delete(path).AssertNotFound()
		d.Owner.PatchJSON(path, testkit.JSON{"name": "X"}).AssertNotFound()
		assert.Len(t, created.All(), 1)
		assert.Len(t, deleted.All(), 1)
	})

	t.Run("refuses a name another tag holds in any case", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Billing"}).AssertCreated()
		other := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Refunds"}).AssertCreated()

		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).
			AssertStatus(409).AssertJSONPath("code", "tag_exists")
		d.Owner.PatchJSON("/api/v1/tags/"+other.String("data.id"), testkit.JSON{"name": "BILLING"}).
			AssertStatus(409).AssertJSONPath("code", "tag_exists")
		d.Owner.PatchJSON("/api/v1/tags/"+other.String("data.id"), testkit.JSON{"name": "  "}).
			AssertStatus(422).AssertJSONPath("code", "tag_name_required")
		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "  "}).
			AssertStatus(422).AssertJSONPath("code", "tag_name_required")
		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "x", "color": "red"}).AssertValidationError("color")
	})

	t.Run("lets members add tags but not rename or remove them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		res := member.PostJSON("/api/v1/tags", testkit.JSON{"name": "VIP"}).AssertCreated()
		path := "/api/v1/tags/" + res.String("data.id")

		member.GetJSON("/api/v1/tags").AssertOK().AssertJSONPath("data.#", 1)
		member.PatchJSON(path, testkit.JSON{"name": "Big"}).AssertForbidden()
		member.Delete(path).AssertForbidden()
		d.Owner.PatchJSON(path, testkit.JSON{"name": "Big"}).AssertOK()
	})

	t.Run("enforces token abilities", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		reader := d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read")

		reader.GetJSON("/api/v1/tags").AssertOK()
		reader.PostJSON("/api/v1/tags", testkit.JSON{"name": "VIP"}).AssertForbidden()
	})

	t.Run("does not reach another workspace's tags", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)

		res := other.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Theirs"}).AssertCreated()
		path := "/api/v1/tags/" + res.String("data.id")

		d.Owner.PatchJSON(path, testkit.JSON{"name": "Mine"}).AssertNotFound()
		d.Owner.Delete(path).AssertNotFound()
		d.Owner.Delete("/api/v1/tags/not-a-tag").AssertNotFound()
		d.Owner.PatchJSON("/api/v1/tags/not-a-tag", testkit.JSON{"name": "Mine"}).AssertNotFound()
	})

	t.Run("filters by prefix and counts live records per tag", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		contact := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ann"}).AssertCreated().String("data.id")
		company := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).AssertCreated().String("data.id")

		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"billing"}}).AssertOK()
		d.Owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"add_tags": []string{"billing", "vip"}}).AssertOK()
		d.Owner.PatchJSON("/api/v1/companies/"+company, testkit.JSON{"add_tags": []string{"billing"}}).AssertOK()

		d.Owner.GetJSON("/api/v1/tags?q=bil").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "billing").
			AssertJSONMissing("data.0.counts")
		d.Owner.GetJSON("/api/v1/tags?include=counts").AssertOK().
			AssertJSONPath("data.0.counts.conversations", 1).
			AssertJSONPath("data.0.counts.contacts", 1).
			AssertJSONPath("data.0.counts.companies", 1).
			AssertJSONPath("data.1.name", "vip").
			AssertJSONPath("data.1.counts.conversations", 0)
	})

	t.Run("deleting a tag cascades and says one event with its counts", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		deleted := tests.Record[appevents.TagDeleted](d.TC)
		conv := d.Customer(t, "bob", "Hi").String("data.id")

		res := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"bug"}}).AssertOK()
		id := res.String("data.tags.0.id")

		d.Owner.Delete("/api/v1/tags/" + id).AssertNoContent()

		d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.tags.#", 0)
		require.Len(t, deleted.All(), 1)
		assert.Equal(t, int64(1), deleted.All()[0].Conversations)
	})

	t.Run("stops new tags at the limit of the vocabulary", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		_, err := d.TC.App.DB.Exec(t.Context(), `
			INSERT INTO tags (workspace_id, name) SELECT $1, 'tag-' || g FROM generate_series(1, 10000) AS g`, ws)
		require.NoError(t, err)

		conv := d.Customer(t, "bob", "Hi").String("data.id")

		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "one more"}).
			AssertStatus(422).AssertJSONPath("code", "tag_limit_reached")
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"one more"}}).
			AssertStatus(422).AssertJSONPath("code", "tag_limit_reached")
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"TAG-7"}}).AssertOK().
			AssertJSONPath("data.tags.0.name", "tag-7")
	})
}

func TestConversationTags(t *testing.T) {
	t.Parallel()

	t.Run("adds by id or name, creating new names, once", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		tagged := tests.Record[appevents.ConversationTagged](d.TC)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		existing := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Known"}).AssertCreated().String("data.id")
		path := "/api/v1/conversations/" + conv + "/tags"

		d.Owner.PostJSON(path, testkit.JSON{"tags": []string{existing, "brand new", "BRAND NEW"}}).AssertOK().
			AssertJSONPath("data.tags.#", 2).
			AssertJSONPath("data.tags.0.name", "brand new").
			AssertJSONPath("data.tags.1.name", "Known")

		d.Owner.PostJSON(path, testkit.JSON{"tags": []string{"known"}}).AssertOK().AssertJSONPath("data.tags.#", 2)
		assert.Len(t, tagged.All(), 1)

		timeline := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/messages?filter[type]=event").AssertOK().Body
		assert.Contains(t, string(timeline), "conversation.tagged")
		assert.Contains(t, string(timeline), "brand new")
	})

	t.Run("notes the message that prompted the tag", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		res := d.Customer(t, "bob", "Hi")
		conv := res.String("data.id")
		other := d.Customer(t, "ann", "Other").String("data.last_message.id")
		message := res.String("data.last_message.id")

		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{
			"tags": []string{"bug"}, "message_id": message,
		}).AssertOK()

		var stored *string

		err := d.TC.App.DB.QueryRow(t.Context(), `SELECT message_id::text FROM conversation_tag LIMIT 1`).Scan(&stored)
		require.NoError(t, err)
		require.NotNil(t, stored)

		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{
			"tags": []string{"feature"}, "message_id": other,
		}).AssertStatus(422).AssertJSONPath("code", "message_not_in_conversation")
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{
			"tags": []string{"feature"}, "message_id": "bogus",
		}).AssertStatus(422).AssertJSONPath("code", "message_not_in_conversation")
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{
			"tags": []string{"feature"}, "message_id": models.PublicID(models.PrefixMessage, d.Inbox.Inbox.ID),
		}).AssertStatus(422).AssertJSONPath("code", "message_not_in_conversation")
	})

	t.Run("refuses past fifty tags and adds none", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		path := "/api/v1/conversations/" + conv + "/tags"
		names := make([]string, 50)

		for i := range names {
			names[i] = fmt.Sprintf("tag-%d", i)
		}

		d.Owner.PostJSON(path, testkit.JSON{"tags": names}).AssertOK().AssertJSONPath("data.tags.#", 50)
		d.Owner.PostJSON(path, testkit.JSON{"tags": []string{"overflow"}}).
			AssertStatus(422).AssertJSONPath("code", "tag_limit_reached")
		d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.tags.#", 50)
	})

	t.Run("removes a tag, and removing one not on it changes nothing", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		untagged := tests.Record[appevents.ConversationUntagged](d.TC)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		path := "/api/v1/conversations/" + conv + "/tags"
		id := d.Owner.PostJSON(path, testkit.JSON{"tags": []string{"bug"}}).AssertOK().String("data.tags.0.id")

		d.Owner.Delete(path+"/"+id).AssertOK().AssertJSONPath("data.tags.#", 0)
		d.Owner.Delete(path+"/"+id).AssertOK().AssertJSONPath("data.tags.#", 0)
		assert.Len(t, untagged.All(), 1)

		d.Owner.Delete(path + "/bogus").AssertNotFound()
		d.Owner.Delete("/api/v1/conversations/bogus/tags/" + id).AssertNotFound()
		d.Owner.PostJSON("/api/v1/conversations/bogus/tags", testkit.JSON{"tags": []string{"x"}}).AssertNotFound()
	})

	t.Run("changes tags through the conversation's patch, adding before removing", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		path := "/api/v1/conversations/" + conv
		id := d.Owner.PostJSON(path+"/tags", testkit.JSON{"tags": []string{"old"}}).AssertOK().String("data.tags.0.id")

		res := d.Owner.PatchJSON(path, testkit.JSON{
			"add_tags": []string{"new"}, "remove_tags": []string{id, "bogus"}, "subject": "Changed",
		}).AssertOK().
			AssertJSONPath("data.tags.#", 1).
			AssertJSONPath("data.tags.0.name", "new")
		res.AssertJSONPath("meta.changed.0", "subject").AssertJSONPath("meta.changed.1", "add_tags").
			AssertJSONPath("meta.changed.2", "remove_tags")

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"new"}}).AssertOK().
			AssertJSONPath("meta.changed.#", 0)
	})

	t.Run("one failing field undoes the tags", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		path := "/api/v1/conversations/" + conv

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"bug"}, "unread": true, "inbox_id": "bogus"}).
			AssertStatus(422)
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.tags.#", 0)
	})

	t.Run("filters lists by all tags or any tag", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		both := d.Customer(t, "bob", "Both").String("data.id")
		one := d.Customer(t, "ann", "One").String("data.id")
		d.Customer(t, "cal", "None")

		a := d.Owner.PostJSON("/api/v1/conversations/"+both+"/tags", testkit.JSON{"tags": []string{"a", "b"}}).
			AssertOK()
		aID, bID := a.String("data.tags.0.id"), a.String("data.tags.1.id")
		d.Owner.PostJSON("/api/v1/conversations/"+one+"/tags", testkit.JSON{"tags": []string{aID}}).AssertOK()

		list := "/api/v1/conversations?filter[view]=all&"
		d.Owner.GetJSON(list+"filter[tag]="+aID).AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON(list+"filter[tag]="+aID+","+bID).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", both)
		d.Owner.GetJSON(list+"filter[tag_any]="+aID+","+bID).AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON(list+"filter[tag]=bogus").AssertStatus(400).AssertJSONPath("code", "invalid_filter")

		six := aID + "," + aID + "," + aID + "," + aID + "," + aID + "," + aID
		d.Owner.GetJSON(list + "filter[tag]=" + six).AssertStatus(400)
	})

	t.Run("tags live on the conversation's list row", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"bug"}}).AssertOK()

		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.tags.0.name", "bug")
	})
}
