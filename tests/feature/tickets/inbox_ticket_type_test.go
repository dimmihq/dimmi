package tickets_test

import (
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestInboxTicketType(t *testing.T) {
	t.Parallel()

	t.Run("sets, shows and clears the type new conversations become", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		inbox := "/api/v1/inboxes/" + models.PublicID(models.PrefixInbox, d.Inbox.Inbox.ID)
		question := d.types["Question"]

		d.Owner.GetJSON(inbox).AssertOK().AssertJSONPath("data.ticket_type_id", nil)
		d.Owner.PatchJSON(inbox, testkit.JSON{"ticket_type_id": question}).AssertOK().
			AssertJSONPath("data.ticket_type_id", question)
		d.Owner.PatchJSON(inbox, testkit.JSON{"name": "Support"}).AssertOK().
			AssertJSONPath("data.ticket_type_id", question)
		d.Owner.GetJSON(inbox).AssertOK().AssertJSONPath("data.ticket_type_id", question)
		d.Owner.PatchJSON(inbox, testkit.JSON{"ticket_type_id": nil}).AssertOK().
			AssertJSONPath("data.ticket_type_id", nil)
	})

	t.Run("refuses a type that is not a live customer type", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		inbox := "/api/v1/inboxes/" + models.PublicID(models.PrefixInbox, d.Inbox.Inbox.ID)

		for _, id := range []string{d.types["Tracker"], d.types["Back-office task"], "tktt_nope"} {
			d.Owner.PatchJSON(inbox, testkit.JSON{"ticket_type_id": id}).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "ticket_type_invalid")
		}
	})
}
