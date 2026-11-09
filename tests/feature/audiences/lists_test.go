package audiences_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
upload sends a CSV list to tag customers with.
*/
func upload(d tests.Desk, tag, csv string) *testkit.Response {
	return d.Owner.PostForm("/api/v1/contact-lists",
		testkit.FormFile{Field: "file", Name: "list.csv", Contents: []byte(csv)}, map[string]string{"tag": tag})
}

func TestContactLists(t *testing.T) {
	t.Parallel()

	t.Run("tags the customers a list names by email, counting the rest", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{})
		contact(d, "bo@acme.dev", testkit.JSON{})

		res := upload(d, "Event 2026", "Name,Email\nAna,ANA@acme.dev\nCy,cy@nowhere.dev\nAna again,ana@acme.dev\n").
			AssertOK()
		res.AssertJSONPath("data.rows", float64(2)).AssertJSONPath("data.matched", float64(1)).
			AssertJSONPath("data.tagged", float64(1)).AssertJSONPath("data.unmatched", float64(1)).
			AssertJSONPath("data.unmatched_sample.0", "cy@nowhere.dev").AssertJSONPath("data.tag_name", "Event 2026")

		tag := res.String("data.tag_id")
		d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{
			"rules": testkit.JSON{"rules": leaf("contact.tags", "has", tag)},
		}).AssertOK().AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.sample.0.email", "ana@acme.dev")

		upload(d, "Event 2026", "email\nana@acme.dev\n").AssertOK().AssertJSONPath("data.tagged", float64(0))
	})

	t.Run("matches by user id and refuses a list it cannot read", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"external_id": "u_7", "name": "Di"}).AssertCreated()

		upload(d, "Owned", "user_id\nu_7\nu_8\n").AssertOK().AssertJSONPath("data.matched", float64(1))
		upload(d, "Owned", "name\nDi\n").AssertStatus(422).AssertJSONPath("code", "list_column_missing")
		upload(d, "", "email\na@b.c\n").AssertStatus(422).AssertJSONPath("code", "tag_name_invalid")
	})
}

func TestFreezingAudiences(t *testing.T) {
	t.Parallel()

	t.Run("tags who matches now, and the tag stays when the audience changes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", testkit.JSON{"plan": "old"})
		contact(d, "bo@acme.dev", testkit.JSON{"plan": "new"})
		id, _ := save(t, d, "Old plan", testkit.JSON{"rules": leaf("contact.custom.plan", "eq", "old")})

		res := d.Owner.PostJSON("/api/v1/audiences/"+id+"/freezes", testkit.JSON{"tag": "Old plan on price day"}).
			AssertOK().AssertJSONPath("data.matched", float64(1)).AssertJSONPath("data.tagged", float64(1))
		tag := res.String("data.tag_id")

		d.Owner.PatchJSON("/api/v1/contacts/"+ana, testkit.JSON{"custom_attributes": testkit.JSON{"plan": "new"}}).
			AssertOK()
		d.Owner.PostJSON("/api/v1/audiences/"+id+"/previews", nil).AssertOK().AssertJSONPath("data.count", float64(0))
		d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{
			"rules": testkit.JSON{"rules": leaf("contact.tags", "has", tag)},
		}).AssertOK().AssertJSONPath("data.count", float64(1))
		assert.NotEmpty(t, tag)
	})
}
