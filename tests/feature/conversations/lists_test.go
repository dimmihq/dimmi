package conversations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestConversationsIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists each view, newest activity first", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, bobMember := d.join(t, enums.RoleMember)
		first := d.customer(t, "ann", "First", "One")
		d.tc.Clock.Travel(time.Minute)
		second := d.customer(t, "cid", "Second", "Two")
		d.tc.Clock.Travel(time.Minute)
		third := d.customer(t, "dee", "Third", "Three")

		d.owner.PatchJSON("/api/v1/conversations/"+first, testkit.JSON{"assignee_id": "me"}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+second, testkit.JSON{"assignee_id": memberID(bobMember)}).AssertOK()

		assert.Equal(t, []string{first}, listed(d.owner.GetJSON("/api/v1/conversations").AssertOK()))
		assert.Equal(t, []string{second}, listed(bob.GetJSON("/api/v1/conversations?filter[view]=mine").AssertOK()))
		assert.Equal(t, []string{third}, listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=unassigned")))
		assert.Equal(t, []string{third, second, first},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=all")))
		assert.Equal(t, []string{first, second, third},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=all&sort=created_at")))
		assert.Equal(t, []string{second},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=member:"+memberID(bobMember))))
		assert.Equal(t, []string{second},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[assignee]="+memberID(bobMember))))
		assert.Equal(t, []string{third},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[assignee]=none")))
		assert.Equal(t, []string{first},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[assignee]=me")))
		assert.Empty(t, listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=mentions")))
	})

	t.Run("narrows by status, inbox, contact, priority and unread", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		open1 := d.customer(t, "ann", "Open", "One")
		closed := d.customer(t, "bea", "Closed", "Two")
		d.owner.PatchJSON("/api/v1/conversations/"+closed, testkit.JSON{"status": "closed", "priority": true}).
			AssertOK()
		contact := d.owner.GetJSON("/api/v1/conversations/" + open1).String("data.contact.id")
		all := "/api/v1/conversations?filter[view]=all"

		assert.Equal(t, []string{open1}, listed(d.owner.GetJSON(all)))
		assert.Equal(t, []string{closed}, listed(d.owner.GetJSON(all+"&filter[status]=closed")))
		assert.Len(t, listed(d.owner.GetJSON(all+"&filter[status]=any")), 2)
		assert.Len(t, listed(d.owner.GetJSON(all+"&filter[status]=open,closed&filter[inbox]="+d.inboxID())), 2)
		assert.Equal(t, []string{open1}, listed(d.owner.GetJSON(all+"&filter[status]=any&filter[contact]="+contact)))
		assert.Equal(t, []string{closed}, listed(d.owner.GetJSON(all+"&filter[status]=any&filter[priority]=true")))
		assert.Equal(t, []string{open1}, listed(d.owner.GetJSON(all+"&filter[unread]=true")))

		d.owner.PatchJSON("/api/v1/conversations/"+open1+"/read-state", testkit.JSON{}).AssertOK()
		assert.Empty(t, listed(d.owner.GetJSON(all+"&filter[unread]=true")))
	})

	t.Run("pages with a cursor tied to its list", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		for _, name := range []string{"a", "b", "c"} {
			d.customer(t, name, "Hello", "Hi")
			d.tc.Clock.Travel(time.Second)
		}

		all := "/api/v1/conversations?filter[view]=all&limit=2"
		page := d.owner.GetJSON(all).AssertOK().AssertJSONPath("data.#", 2)
		cursor := page.String("meta.next_cursor")

		next := d.owner.GetJSON(all+"&cursor="+cursor).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("meta.next_cursor", nil)
		assert.NotContains(t, listed(page), listed(next)[0])

		d.owner.GetJSON("/api/v1/conversations?filter[view]=unassigned&limit=2&cursor="+cursor).
			AssertStatus(400).AssertJSONPath("code", "invalid_cursor")
		d.owner.GetJSON(all + "&cursor=forged").AssertStatus(400)
	})

	t.Run("refuses filters and sorts no list has", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		for _, query := range []string{
			"filter[view]=trash", "filter[view]=member:nope", "filter[status]=done", "filter[inbox]=nope",
			"filter[assignee]=nope", "filter[priority]=maybe", "filter[unread]=1", "filter[tag]=x", "sort=-priority",
		} {
			d.owner.GetJSON("/api/v1/conversations?" + query).AssertStatus(400)
		}
	})

	t.Run("finds conversations by words, numbers and is filters", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		refund := d.customer(t, "ann", "Card declined", "I want a refund for the duplicate charge")
		export := d.customer(t, "bea", "Export", "The CSV export spins forever")
		d.owner.PatchJSON("/api/v1/conversations/"+export, testkit.JSON{"status": "closed"}).AssertOK()
		all := "/api/v1/conversations?filter[view]=all&q="

		assert.Equal(t, []string{refund}, listed(d.owner.GetJSON(all+"refunds")))
		assert.Equal(t, []string{refund}, listed(d.owner.GetJSON(all+"declin")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"export")))
		assert.Equal(t, []string{export}, listed(d.owner.GetJSON(all+"export%20is:closed")))
		assert.Equal(t, []string{export}, listed(d.owner.GetJSON(all+"%232")))
		assert.Equal(t, []string{refund}, listed(d.owner.GetJSON(all+"%22duplicate%20charge%22")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"refund%20-duplicate")))
		assert.Len(t, listed(d.owner.GetJSON(all+"the")), 1)
		assert.Empty(t, listed(d.owner.GetJSON(all+export)))
		assert.Equal(t, []string{refund}, listed(d.owner.GetJSON(all+"is:unread%20is:open%20is:unassigned")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"is:mine")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"is:priority")))
	})

	t.Run("finds conversations by the customer's name or email", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		rosa := d.customer(t, "rosalind", "Where is my order", "Nothing shipped yet")
		d.customer(t, "bea", "Export", "The CSV export spins forever")
		all := "/api/v1/conversations?filter[view]=all&q="

		assert.Equal(t, []string{rosa}, listed(d.owner.GetJSON(all+"rosalind")))
		assert.Equal(t, []string{rosa}, listed(d.owner.GetJSON(all+"rosa")))
		assert.Equal(t, []string{rosa}, listed(d.owner.GetJSON(all+"rosalind@example.com")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"rosalind%20export")))
	})
}

func TestConversationNeighbors(t *testing.T) {
	t.Parallel()

	t.Run("finds the conversations around one, even after it left the list", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		var all []string
		for _, name := range []string{"a", "b", "c"} {
			all = append(all, d.customer(t, name, "Hello", "Hi"))
			d.tc.Clock.Travel(time.Second)
		}

		view := "/neighbors?filter[view]=all"
		d.owner.GetJSON("/api/v1/conversations/"+all[1]+view).AssertOK().
			AssertJSONPath("data.object", "conversation_neighbors").
			AssertJSONPath("data.prev.id", all[2]).
			AssertJSONPath("data.next.id", all[0])
		d.owner.GetJSON("/api/v1/conversations/"+all[2]+view).AssertOK().AssertJSONPath("data.prev", nil)
		d.owner.GetJSON("/api/v1/conversations/"+all[0]+view).AssertOK().AssertJSONPath("data.next", nil)

		d.owner.PatchJSON("/api/v1/conversations/"+all[1], testkit.JSON{"status": "closed"}).AssertOK()
		d.owner.GetJSON("/api/v1/conversations/"+all[1]+view).AssertOK().AssertJSONPath("data.next.id", all[0])
		d.owner.GetJSON("/api/v1/conversations/"+all[1]+"/neighbors?filter[view]=mentions").AssertOK().
			AssertJSONPath("data.next", nil)
		d.owner.GetJSON("/api/v1/conversations/" + all[1] + "/neighbors?filter[view]=nope").AssertStatus(400)
		d.owner.GetJSON("/api/v1/conversations/conv_nope/neighbors").AssertNotFound()
	})
}

func TestConversationCounts(t *testing.T) {
	t.Parallel()

	t.Run("counts each view for the caller", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		mine := d.customer(t, "a", "Mine", "Hi")
		snoozed := d.customer(t, "b", "Snoozed", "Hi")
		d.customer(t, "c", "Waiting", "Hi")
		spam := d.customer(t, "d", "Spam", "Buy now")
		d.owner.PatchJSON("/api/v1/conversations/"+mine, testkit.JSON{"assignee_id": "me"}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+snoozed, testkit.JSON{
			"assignee_id": "me", "status": "snoozed", "snooze_preset": "later_today",
		}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+spam, testkit.JSON{"spam": true}).AssertOK()

		d.owner.GetJSON("/api/v1/conversation-counts").AssertOK().
			AssertJSONPath("data.object", "conversation_counts").
			AssertJSONPath("data.views.mine.open", 1).
			AssertJSONPath("data.views.mine.unread", 1).
			AssertJSONPath("data.views.mine.snoozed", 1).
			AssertJSONPath("data.views.unassigned.open", 1).
			AssertJSONPath("data.views.unassigned.unread", 1).
			AssertJSONPath("data.views.all.open", 2).
			AssertJSONPath("data.views.all.capped", false).
			AssertJSONPath("data.views.spam.total", 1).
			AssertJSONPath("data.views.inboxes."+d.inboxID()+".open", 2)
	})

	t.Run("caps large counts", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer(t, "a", "Hi", "Hi")
		_, err := d.tc.App.DB.Exec(t.Context(), `
			INSERT INTO conversations (workspace_id, number, inbox_id, channel_connection_id, contact_id)
			SELECT workspace_id, number + n, inbox_id, channel_connection_id, contact_id
			FROM conversations, generate_series(1, 1000) AS n`)
		if err != nil {
			t.Fatal(err)
		}

		d.owner.GetJSON("/api/v1/conversation-counts").AssertOK().
			AssertJSONPath("data.views.all.open", 999).
			AssertJSONPath("data.views.all.capped", true)
	})
}

func TestConversationReadState(t *testing.T) {
	t.Parallel()

	t.Run("reads up to a message, forward only, and marks unread", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "First")
		first := d.owner.GetJSON(path + "/messages").String("data.0.id")
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Second"}).AssertCreated()
		second := d.owner.GetJSON(path + "/messages").String("data.1.id")

		d.owner.GetJSON(path+"/read-state").AssertOK().
			AssertJSONPath("data.object", "conversation_read_state").
			AssertJSONPath("data.last_read_message_id", nil).
			AssertJSONPath("data.unread", true)

		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"last_read_message_id": second}).AssertOK().
			AssertJSONPath("data.last_read_message_id", second).
			AssertJSONPath("data.unread", false)
		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"last_read_message_id": first}).AssertOK().
			AssertJSONPath("data.last_read_message_id", second)

		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"unread": true}).AssertOK().
			AssertJSONPath("data.last_read_message_id", first).
			AssertJSONPath("data.unread", true)
		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"unread": false}).AssertOK().
			AssertJSONPath("data.unread", false)
	})

	t.Run("refuses a message of another conversation and both keys at once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "First")
		other := "/api/v1/conversations/" + d.customer(t, "bea", "Other", "Elsewhere")
		elsewhere := d.owner.GetJSON(other + "/messages").String("data.0.id")

		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"last_read_message_id": elsewhere}).
			AssertStatus(422).AssertJSONPath("code", "message_not_in_conversation")
		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"last_read_message_id": "nope"}).AssertStatus(422)
		d.owner.PatchJSON(path+"/read-state", testkit.JSON{"last_read_message_id": elsewhere, "unread": true}).
			AssertValidationError("unread")
		d.owner.GetJSON("/api/v1/conversations/conv_nope/read-state").AssertNotFound()
	})
}

/*
listed returns the ids of the conversations a list answered with.
*/
func listed(res *testkit.Response) []string {
	return sliceOf(res, "data.#.id")
}
