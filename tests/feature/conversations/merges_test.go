package conversations_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

func TestBulkConversationChanges(t *testing.T) {
	t.Parallel()

	t.Run("closes, assigns and tags many conversations and says how each went", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		one := d.customer(t, "Ana", "One", "Hi.")
		two := d.customer(t, "Bo", "Two", "Hi.")

		res := d.owner.PostJSON("/api/v1/conversation-bulk-changes", testkit.JSON{
			"conversation_ids": []string{one, two, "conv_bogus"},
			"changes":          testkit.JSON{"status": "closed", "assignee_id": "me", "add_tags": []string{"bulk"}},
		}).AssertOK().AssertJSONPath("data.#", 3)

		res.AssertJSONPath("data.0.id", "conv_bogus").AssertJSONPath("data.0.error", "not_found")
		res.AssertJSONPath("data.1.id", one).AssertJSONPath("data.1.ok", true)
		assert.Contains(t, res.String("data.1.changed"), "status")

		for _, id := range []string{one, two} {
			d.owner.GetJSON("/api/v1/conversations/"+id).AssertOK().
				AssertJSONPath("data.status", "closed").
				AssertJSONPath("data.assignee.id", d.ownerID()).
				AssertJSONPath("data.tags.0.name", "bulk")
		}
	})

	t.Run("marks many as spam, unread, and moves them to the trash", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		one := d.customer(t, "Ana", "One", "Hi.")
		two := d.customer(t, "Bo", "Two", "Hi.")
		ids := []string{one, two}

		d.owner.PostJSON("/api/v1/conversation-bulk-changes", testkit.JSON{
			"conversation_ids": ids, "changes": testkit.JSON{"spam": true, "unread": true},
		}).AssertOK().AssertJSONPath("data.0.ok", true).AssertJSONPath("data.1.ok", true)
		d.owner.GetJSON("/api/v1/conversations/"+one).AssertOK().
			AssertJSONPath("data.spam", true).AssertJSONPath("data.unread", true)

		d.owner.PostJSON("/api/v1/conversation-bulk-changes", testkit.JSON{"conversation_ids": ids, "trash": true}).
			AssertOK().AssertJSONPath("data.1.changed.0", "deleted")
		d.owner.GetJSON("/api/v1/conversations/" + two).AssertNotFound()
	})

	t.Run("refuses a body without one action, and past the cap", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		one := d.customer(t, "Ana", "One", "Hi.")
		path := "/api/v1/conversation-bulk-changes"

		d.owner.PostJSON(path, testkit.JSON{"conversation_ids": []string{one}}).AssertStatus(422)
		d.owner.PostJSON(path, testkit.JSON{
			"conversation_ids": []string{one}, "trash": true, "changes": testkit.JSON{"status": "closed"},
		}).AssertStatus(422)
		d.owner.PostJSON(path, testkit.JSON{
			"conversation_ids": []string{one}, "changes": testkit.JSON{"snooze_preset": "tomorrow"},
		}).AssertStatus(422)

		many := make([]string, 201)
		for i := range many {
			many[i] = fmt.Sprintf("conv_%d", i)
		}

		d.owner.PostJSON(path, testkit.JSON{"conversation_ids": many, "trash": true}).AssertStatus(422)
	})

	t.Run("keeps the trash and writes to those allowed", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		one := d.customer(t, "Ana", "One", "Hi.")
		member, _ := d.join(t, enums.RoleMember)
		path := "/api/v1/conversation-bulk-changes"

		member.PostJSON(path, testkit.JSON{"conversation_ids": []string{one}, "trash": true}).AssertForbidden()
		reader := d.tc.ActingAsToken(d.tc.Workspace.Owner, "conversations:read")
		reader.PostJSON(path, testkit.JSON{
			"conversation_ids": []string{one}, "changes": testkit.JSON{"status": "closed"},
		}).AssertForbidden()
		d.owner.GetJSON("/api/v1/conversations/"+one).AssertOK().AssertJSONPath("data.status", "open")
	})
}

func TestMergeConversations(t *testing.T) {
	t.Parallel()

	t.Run("moves the sources' messages into the target in order and trashes the sources", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		merged := record[appevents.ConversationMerged](d.tc)
		older := d.customer(t, "Ana", "First", "My order is late.")
		target := d.customer(t, "Ana", "Second", "Any news?")

		d.owner.PostJSON("/api/v1/conversations/"+target+"/merges", testkit.JSON{
			"source_ids": []string{older},
		}).AssertOK().AssertJSONPath("data.id", target).AssertJSONPath("data.message_count", 2)

		d.owner.GetJSON("/api/v1/conversations/"+older).AssertNotFound().
			AssertJSONPath("code", "merged_into").
			AssertJSONPath("target_id", target)

		timeline := d.owner.GetJSON("/api/v1/conversations/" + target + "/messages").AssertOK()
		bodies := []string{}

		count, err := strconv.Atoi(timeline.String("data.#"))
		require.NoError(t, err)

		for i := range count {
			if timeline.String(fmt.Sprintf("data.%d.type", i)) == "message" {
				bodies = append(bodies, timeline.String(fmt.Sprintf("data.%d.body", i)))
			}
		}

		assert.Equal(t, []string{"My order is late.", "Any news?"}, bodies)
		assert.Contains(t, string(timeline.Body), "conversation.merged")

		all := merged.all()
		require.Len(t, all, 1)
		assert.Equal(t, 1, all[0].MovedMessages)
		d.tc.Table("conversations").AssertCount(1, testkit.Row{"status": "closed", "message_count": 0})
	})

	t.Run("leads a merged conversation's number to the target, and stops at a trashed one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		older := d.customer(t, "Ana", "First", "Hi.")
		number := d.owner.GetJSON("/api/v1/conversations/" + older).AssertOK().String("data.number")
		target := d.customer(t, "Ana", "Second", "Hi again.")
		newest := d.customer(t, "Ana", "Third", "Still there?")

		d.owner.PostJSON("/api/v1/conversations/"+target+"/merges", testkit.JSON{"source_ids": []string{older}}).
			AssertOK()
		d.owner.PostJSON("/api/v1/conversations/"+newest+"/merges", testkit.JSON{"source_ids": []string{target}}).
			AssertOK()

		d.owner.GetJSON("/api/v1/conversations/"+number).AssertNotFound().
			AssertJSONPath("code", "merged_into").
			AssertJSONPath("target_id", newest)

		d.owner.Delete("/api/v1/conversations/" + newest).AssertNoContent()
		d.owner.GetJSON("/api/v1/conversations/"+older).AssertNotFound().AssertJSONPath("code", "not_found")
	})

	t.Run("refuses another customer's conversation and the target itself", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		target := d.customer(t, "Ana", "One", "Hi.")
		other := d.customer(t, "Bo", "Two", "Hi.")
		path := "/api/v1/conversations/" + target + "/merges"

		d.owner.PostJSON(path, testkit.JSON{"source_ids": []string{other}}).
			AssertStatus(422).AssertJSONPath("code", "merge_other_customer")
		d.owner.PostJSON(path, testkit.JSON{"source_ids": []string{target}}).
			AssertStatus(422).AssertJSONPath("code", "merge_into_itself")
		d.owner.PostJSON(path, testkit.JSON{"source_ids": []string{"conv_bogus"}}).AssertNotFound()
		d.owner.GetJSON("/api/v1/conversations/" + other).AssertOK()
	})

	t.Run("merges people at the same company", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		target := d.customer(t, "Ana", "One", "Hi.")
		other := d.customer(t, "Bo", "Two", "Hello.")
		company := d.owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).AssertCreated().String("data.id")

		for _, id := range []string{target, other} {
			contact := d.owner.GetJSON("/api/v1/conversations/" + id).AssertOK().String("data.contact.id")
			d.owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"company_id": company}).AssertOK()
		}

		d.owner.PostJSON("/api/v1/conversations/"+target+"/merges", testkit.JSON{"source_ids": []string{other}}).
			AssertOK()
		d.owner.GetJSON("/api/v1/conversations/" + other).AssertNotFound()
	})

	t.Run("keeps merging to those who may write", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		target := d.customer(t, "Ana", "One", "Hi.")
		older := d.customer(t, "Ana", "Two", "Hi.")
		reader := d.tc.ActingAsToken(d.tc.Workspace.Owner, "conversations:read")

		reader.PostJSON("/api/v1/conversations/"+target+"/merges", testkit.JSON{"source_ids": []string{older}}).
			AssertForbidden()
		d.owner.GetJSON("/api/v1/conversations/" + older).AssertOK()
	})
}
