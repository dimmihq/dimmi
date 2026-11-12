package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

func TestConversationTrash(t *testing.T) {
	t.Parallel()

	t.Run("moves a conversation to the trash and puts it back", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		deleted := record[appevents.ConversationDeleted](d.tc)
		id := d.customer(t, "Ana", "Refund", "Please refund me.")
		path := "/api/v1/conversations/" + id

		d.owner.Delete(path).AssertNoContent()
		d.owner.GetJSON(path).AssertNotFound()
		d.owner.Delete(path).AssertNotFound()
		assert.Len(t, deleted.all(), 1)

		d.owner.PostJSON(path+"/restore", nil).AssertOK().AssertJSONPath("data.id", id)
		d.owner.GetJSON(path).AssertOK()
		d.owner.PostJSON(path+"/restore", nil).AssertNotFound()
	})

	t.Run("keeps the trash to those who may delete", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "Bo", "Help", "Hello.")
		member, _ := d.join(t, enums.RoleMember)

		member.Delete("/api/v1/conversations/" + id).AssertForbidden()
		d.owner.Delete("/api/v1/conversations/" + id).AssertNoContent()
		member.PostJSON("/api/v1/conversations/"+id+"/restore", nil).AssertForbidden()
	})
}
