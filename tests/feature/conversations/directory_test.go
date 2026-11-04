package conversations_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

func TestInboxes(t *testing.T) {
	t.Parallel()

	t.Run("lists, shows, adds and renames inboxes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		created := record[appevents.InboxCreated](d.tc)
		updated := record[appevents.InboxUpdated](d.tc)

		sales := d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertCreated().
			AssertJSONPath("data.object", "inbox").
			AssertJSONPath("data.name", "Sales").
			String("data.id")
		d.owner.GetJSON("/api/v1/inboxes").AssertOK().
			AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.name", "Support").
			AssertJSONPath("meta.per_page", 200)
		d.owner.GetJSON("/api/v1/inboxes/"+sales).AssertOK().AssertJSONPath("data.id", sales)
		d.owner.PatchJSON("/api/v1/inboxes/"+sales, testkit.JSON{"name": "Revenue"}).AssertOK().
			AssertJSONPath("data.name", "Revenue")
		d.owner.PatchJSON("/api/v1/inboxes/"+sales, testkit.JSON{"name": "Revenue"}).AssertOK()
		d.owner.PatchJSON("/api/v1/inboxes/"+sales, testkit.JSON{}).AssertOK()

		assert.Len(t, created.all(), 1)
		require.Len(t, updated.all(), 1)
		assert.Equal(t, "Sales", updated.all()[0].FromName)
	})

	t.Run("refuses blank names and other workspaces' inboxes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := open(t)

		d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": " \u200b "}).AssertValidationError("name")
		d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": strings.Repeat("x", 81)}).AssertValidationError("name")
		d.owner.PatchJSON("/api/v1/inboxes/"+d.inboxID(), testkit.JSON{"name": " "}).AssertValidationError("name")
		other.owner.GetJSON("/api/v1/inboxes/" + d.inboxID()).AssertNotFound()
		other.owner.PatchJSON("/api/v1/inboxes/"+d.inboxID(), testkit.JSON{"name": "Mine"}).AssertNotFound()
		d.owner.GetJSON("/api/v1/inboxes/inbox_nope").AssertNotFound()
		d.owner.PatchJSON("/api/v1/inboxes/inbox_nope", testkit.JSON{"name": "x"}).AssertNotFound()
	})

	t.Run("lets admins manage inboxes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		admin, _ := d.join(t, enums.RoleAdmin)

		admin.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertCreated()
		admin.PatchJSON("/api/v1/inboxes/"+d.inboxID(), testkit.JSON{"name": "Help"}).AssertOK()
	})

	t.Run("lets members read inboxes but not manage them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, m := d.join(t, enums.RoleMember)

		member.GetJSON("/api/v1/inboxes").AssertOK().AssertJSONPath("data.#", 1)
		member.GetJSON("/api/v1/inboxes/" + d.inboxID()).AssertOK()
		member.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertForbidden()
		member.PatchJSON("/api/v1/inboxes/"+d.inboxID(), testkit.JSON{"name": "Mine"}).AssertForbidden()

		token := d.tc.ActingAsToken(m, "contacts:read")
		token.GetJSON("/api/v1/inboxes").AssertForbidden()
		token.GetJSON("/api/v1/inboxes/" + d.inboxID()).AssertForbidden()
	})
}

func TestContacts(t *testing.T) {
	t.Parallel()

	t.Run("adds a contact, users by external id, leads otherwise", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		created := record[appevents.ContactCreated](d.tc)

		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"external_id": "user_42", "email": "Bob@Acme.dev", "name": "Bob", "phone": "+1 (415) 555-0100",
			"custom_attributes": testkit.JSON{"plan": "pro", "seats": 12},
		}).AssertCreated().
			AssertJSONPath("data.object", "contact").
			AssertJSONPath("data.kind", "user").
			AssertJSONPath("data.email", "bob@acme.dev").
			AssertJSONPath("data.email_verified", true).
			AssertJSONPath("data.phone", "+14155550100").
			AssertJSONPath("data.custom_attributes.plan", "pro").
			AssertJSONPath("data.blocked", false)
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Walk-in"}).AssertCreated().
			AssertJSONPath("data.kind", "lead").
			AssertJSONPath("data.email", nil)

		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"external_id": "user_42"}).
			AssertStatus(409).AssertJSONPath("code", "contact_exists")
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"custom_attributes": testkit.JSON{"Bad Key": 1}}).
			AssertStatus(422).AssertJSONPath("code", "invalid_custom_attributes")
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"custom_attributes": testkit.JSON{"nested": testkit.JSON{}}}).
			AssertStatus(422)
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"custom_attributes": testkit.JSON{"long": strings.Repeat("x", 1025)},
		}).AssertStatus(422)
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "not an email"}).AssertValidationError("email")
		assert.Len(t, created.all(), 2)
	})

	t.Run("lists, narrows, searches and pages contacts", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		for _, c := range []testkit.JSON{
			{"name": "Ada Lovelace", "email": "ada@engine.dev", "external_id": "u1"},
			{"name": "Grace Hopper", "email": "grace@navy.mil"},
			{"name": "Alan Turing", "email": "alan@bletchley.uk"},
		} {
			d.owner.PostJSON("/api/v1/contacts", c).AssertCreated()
			d.tc.Clock.Travel(time.Minute)
		}

		names := func(query string) []string {
			return sliceOf(d.owner.GetJSON("/api/v1/contacts"+query).AssertOK(), "data.#.name")
		}

		assert.Equal(t, []string{"Alan Turing", "Grace Hopper", "Ada Lovelace"}, names(""))
		assert.Equal(t, []string{"Ada Lovelace", "Alan Turing", "Grace Hopper"}, names("?sort=name"))
		assert.Equal(t, []string{"Ada Lovelace"}, names("?filter[kind]=user"))
		assert.Equal(t, []string{"Grace Hopper"}, names("?filter[email]=GRACE@navy.mil"))
		assert.Equal(t, []string{"Ada Lovelace"}, names("?filter[external_id]=u1"))
		assert.Equal(t, []string{"Grace Hopper"}, names("?q=hopp"))
		assert.Equal(t, []string{"Alan Turing"}, names("?q=alan@"))
		assert.Empty(t, names("?filter[blocked]=true"))

		page := d.owner.GetJSON("/api/v1/contacts?limit=2&sort=-created_at").AssertOK().AssertJSONPath("data.#", 2)
		d.owner.GetJSON("/api/v1/contacts?limit=2&sort=-created_at&cursor="+page.String("meta.next_cursor")).
			AssertOK().AssertJSONPath("data.#", 1)

		for _, query := range []string{"filter[kind]=robot", "filter[blocked]=maybe", "cursor=forged", "sort=age"} {
			d.owner.GetJSON("/api/v1/contacts?" + query).AssertStatus(400)
		}
	})

	t.Run("changes, clears and blocks a contact, keeping trusted keys in step", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"name": "Bob", "email": "bob@acme.dev", "custom_attributes": testkit.JSON{"plan": "pro", "seats": 2},
		}).AssertCreated().String("data.id")
		updated := record[appevents.ContactUpdated](d.tc)
		path := "/api/v1/contacts/" + id

		d.owner.PatchJSON(path, testkit.JSON{
			"name": "Robert", "phone": nil, "email": nil, "custom_attributes": testkit.JSON{"plan": nil, "seats": 3},
		}).AssertOK().
			AssertJSONPath("data.name", "Robert").
			AssertJSONPath("data.email", nil).
			AssertJSONMissing("data.custom_attributes.plan").
			AssertJSONPath("data.custom_attributes.seats", 3)
		d.owner.PatchJSON(path, testkit.JSON{"blocked": true}).AssertOK().AssertJSONPath("data.blocked", true)
		d.owner.PatchJSON(path, testkit.JSON{"blocked": true}).AssertOK()
		d.owner.PatchJSON(path, testkit.JSON{"external_id": "u9"}).AssertOK().
			AssertJSONPath("data.kind", "user").
			AssertJSONPath("data.external_id", "u9")
		d.owner.PatchJSON(path, testkit.JSON{"external_id": nil, "blocked": false}).AssertOK().
			AssertJSONPath("data.external_id", nil).
			AssertJSONPath("data.blocked", false)

		all := updated.all()
		require.Len(t, all, 4)
		assert.Equal(t, []string{"email", "name", "custom_attributes"}, all[0].Changes)
		assert.Equal(t, []string{"plan", "seats"}, all[0].ChangedAttributeKeys)
		assert.True(t, all[1].Blocked)
		assert.True(t, all[3].Unblocked)

		var trusted []string
		err := d.tc.App.DB.QueryRow(t.Context(), "SELECT trusted_attribute_keys FROM contacts").Scan(&trusted)
		require.NoError(t, err)
		assert.Equal(t, []string{"seats"}, trusted)
	})

	t.Run("keeps external ids unique and to those allowed to manage them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, _ := d.join(t, enums.RoleMember)
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"external_id": "taken"}).AssertCreated()
		id := d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Bob"}).AssertCreated().String("data.id")

		d.owner.PatchJSON("/api/v1/contacts/"+id, testkit.JSON{"external_id": "taken"}).
			AssertStatus(409).AssertJSONPath("code", "contact_exists")
		member.PatchJSON("/api/v1/contacts/"+id, testkit.JSON{"external_id": "u1"}).AssertForbidden()
		member.PatchJSON("/api/v1/contacts/"+id, testkit.JSON{"name": "Rob"}).AssertOK()
		member.PatchJSON("/api/v1/contacts/"+id, testkit.JSON{"custom_attributes": testkit.JSON{"x y": 1}}).
			AssertStatus(422)
	})

	t.Run("shows a contact, following merges, and nothing of another workspace", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := open(t)
		survivor := d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Kept"}).AssertCreated().String("data.id")
		merged := d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Gone"}).AssertCreated().String("data.id")
		_, err := d.tc.App.DB.Exec(t.Context(),
			"UPDATE contacts SET merged_into_id = (SELECT id FROM contacts WHERE name = 'Kept') WHERE name = 'Gone'")
		require.NoError(t, err)

		d.owner.GetJSON("/api/v1/contacts/"+merged).AssertOK().AssertJSONPath("data.id", survivor)
		d.owner.GetJSON("/api/v1/contacts").AssertOK().AssertJSONPath("data.#", 1)
		other.owner.GetJSON("/api/v1/contacts/" + survivor).AssertNotFound()
		other.owner.PatchJSON("/api/v1/contacts/"+survivor, testkit.JSON{"name": "x"}).AssertNotFound()
		d.owner.GetJSON("/api/v1/contacts/ctc_nope").AssertNotFound()
		d.owner.PatchJSON("/api/v1/contacts/ctc_nope", testkit.JSON{"name": "x"}).AssertNotFound()

		_, err = d.tc.App.DB.Exec(t.Context(), "UPDATE contacts SET deleted_at = now()")
		require.NoError(t, err)
		d.owner.GetJSON("/api/v1/contacts/" + survivor).AssertNotFound()
	})

	t.Run("keeps contacts to tokens with contact abilities", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, m := d.join(t, enums.RoleMember)
		token := d.tc.ActingAsToken(m, "conversations:read")

		token.GetJSON("/api/v1/contacts").AssertForbidden()
		token.PostJSON("/api/v1/contacts", testkit.JSON{"name": "x"}).AssertForbidden()
	})
}

func TestSearch(t *testing.T) {
	t.Parallel()

	t.Run("finds conversations with excerpts and contacts", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		refund := d.customer(t, "ann", "Card declined", "I want a <b>refund</b> for the duplicate charge please")
		closed := d.customer(t, "refundbot", "Old one", "Another refund question")
		d.owner.PatchJSON("/api/v1/conversations/"+closed, testkit.JSON{"status": "closed"}).AssertOK()
		spam := d.customer(t, "spammer", "Refund now", "refund now")
		d.owner.PatchJSON("/api/v1/conversations/"+spam, testkit.JSON{"spam": true}).AssertOK()

		res := d.owner.GetJSON("/api/v1/search?q=refund").AssertOK().
			AssertJSONPath("data.object", "search_results").
			AssertJSONPath("data.jump", nil).
			AssertJSONPath("data.conversations.#", 2).
			AssertJSONPath("data.companies", []any{})
		assert.ElementsMatch(t, []string{refund, closed}, sliceOf(res, "data.conversations.#.conversation.id"))
		assert.Contains(t, res.String("data.conversations.#(conversation.id=="+refund+").highlight"),
			"&lt;b&gt;<mark>refund</mark>&lt;/b&gt;")
		assert.Equal(t, []string{"refundbot"}, sliceOf(res, "data.contacts.#.name"))

		d.owner.GetJSON("/api/v1/search?q=refund&limit=1").AssertOK().AssertJSONPath("data.conversations.#", 1)
	})

	t.Run("jumps to a conversation by number or id", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Card declined", "Hello")

		d.owner.GetJSON("/api/v1/search?q=%231").AssertOK().
			AssertJSONPath("data.jump.id", id).
			AssertJSONPath("data.conversations", []any{})
		d.owner.GetJSON("/api/v1/search?q="+id).AssertOK().AssertJSONPath("data.jump.number", 1)
		d.owner.GetJSON("/api/v1/search?q=2").AssertOK().AssertJSONPath("data.jump", nil)
		d.owner.GetJSON("/api/v1/search?q=conv_00000000000000000000000000").AssertOK().AssertJSONPath("data.jump", nil)
		open(t).owner.GetJSON("/api/v1/search?q="+id).AssertOK().AssertJSONPath("data.jump", nil)
	})

	t.Run("refuses long searches and bad limits, and leaves contacts to those who may read them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, m := d.join(t, enums.RoleMember)
		d.customer(t, "ann", "Hello", "Hello")

		d.owner.GetJSON("/api/v1/search?q=" + strings.Repeat("a", 257)).AssertValidationError("q")
		d.owner.GetJSON("/api/v1/search?q=a&limit=0").AssertStatus(400)
		d.owner.GetJSON("/api/v1/search?q=a&limit=x").AssertStatus(400)
		d.owner.GetJSON("/api/v1/search").AssertOK().AssertJSONPath("data.conversations", []any{})

		d.tc.ActingAsToken(m, "conversations:read").GetJSON("/api/v1/search?q=ann").AssertOK().
			AssertJSONPath("data.contacts", []any{})
		d.tc.ActingAsToken(m, "contacts:read").GetJSON("/api/v1/search?q=ann").AssertForbidden()
	})
}
