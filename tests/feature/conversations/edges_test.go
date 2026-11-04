package conversations_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestInboxEdges(t *testing.T) {
	t.Parallel()

	t.Run("reads snooze presets in the person's own time zone", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE users SET timezone = 'America/New_York'")
		require.NoError(t, err)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")

		res := d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "next_week"}).AssertOK()
		assertTime(t, res, "data.snoozed_until", "2026-11-09T14:00:00Z")
		res = d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "one_week"}).AssertOK()
		assertTime(t, res, "data.snoozed_until", "2026-11-09T14:00:00Z")
		res = d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "later_today"}).AssertOK()
		assertTime(t, res, "data.snoozed_until", "2026-11-02T12:00:00Z")
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "someday"}).
			AssertValidationError("snooze_preset")
	})

	t.Run("refuses a reply that would snooze into the past, keeping nothing", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")

		d.owner.PostJSON(path+"/replies", testkit.JSON{
			"body": "x", "then": "snooze", "snooze": testkit.JSON{"until": d.tc.Clock.Now()},
		}).AssertStatus(422).AssertJSONPath("code", "snooze_in_past")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "x", "then": "later"}).AssertValidationError("then")
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("lists what arrived since with long messages cut short", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")
		seen := d.owner.GetJSON(path + "/messages").String("data.0.id")
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": strings.Repeat("long ", 60)}).AssertCreated()

		res := d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "x", "expected_last_message_id": seen}).
			AssertStatus(409)
		require.True(t, strings.HasSuffix(res.String("data.0.excerpt"), "…"))
		d.owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.last_message.excerpt", res.String("data.0.excerpt"))
	})

	t.Run("shows a reply's markdown as plain text in the list", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "We added **decaf**\n\n- one bag\n- [track it](https://x.test)"}).
			AssertCreated().AssertJSONPath("data.body", "We added **decaf**\n\n- one bag\n- [track it](https://x.test)")

		d.owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.last_message.excerpt", "We added decaf one bag track it")
	})

	t.Run("normalizes empty contact details away and caps custom attributes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		attrs := testkit.JSON{}
		for i := range 101 {
			attrs[fmt.Sprintf("key_%d", i)] = i
		}

		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"phone": "call me", "name": "Ann"}).AssertCreated().
			AssertJSONPath("data.phone", nil)
		d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"custom_attributes": attrs}).
			AssertStatus(422).AssertJSONPath("code", "invalid_custom_attributes")
	})

	t.Run("starts a conversation for an existing contact by id", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		contact := d.owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ann", "email": "ann@x.dev"}).
			AssertCreated().String("data.id")

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Hi", "contact": testkit.JSON{"id": contact},
		}).AssertCreated().AssertJSONPath("data.contact.id", contact)
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Hi", "contact": testkit.JSON{"email": "ann@x.dev"},
		}).AssertCreated().AssertJSONPath("data.contact.id", contact)
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Hi", "contact": testkit.JSON{"id": "nope"},
		}).AssertNotFound()
	})
}
