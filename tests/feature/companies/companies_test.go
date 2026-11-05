package companies_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
company adds a company and returns its path.
*/
func company(t *testing.T, d tests.Desk, fields testkit.JSON) string {
	t.Helper()

	return "/api/v1/companies/" + d.Owner.PostJSON("/api/v1/companies", fields).AssertCreated().String("data.id")
}

func TestCompanies(t *testing.T) {
	t.Parallel()

	t.Run("adds, shows, changes and pages companies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		created := tests.Record[appevents.CompanyCreated](d.TC)
		updated := tests.Record[appevents.CompanyUpdated](d.TC)

		path := company(t, d, testkit.JSON{
			"name": "Acme", "domain": "WWW.Acme.Dev", "custom_attributes": testkit.JSON{"seats": 12},
		})
		d.TC.Clock.Travel(time.Minute)
		company(t, d, testkit.JSON{"name": "Globex"})

		d.Owner.GetJSON(path).AssertOK().
			AssertJSONPath("data.object", "company").
			AssertJSONPath("data.domain", "acme.dev").
			AssertJSONPath("data.custom_attributes.seats", 12)

		d.Owner.PatchJSON(path, testkit.JSON{
			"name": "Acme Inc", "domain": nil, "notes": "Key account", "external_id": "org_7",
			"custom_attributes": testkit.JSON{"seats": nil, "plan": "pro"},
		}).AssertOK().
			AssertJSONPath("data.name", "Acme Inc").
			AssertJSONPath("data.domain", nil).
			AssertJSONPath("data.notes", "Key account").
			AssertJSONPath("data.external_id", "org_7").
			AssertJSONPath("data.custom_attributes.plan", "pro").
			AssertJSONMissing("data.custom_attributes.seats")
		d.Owner.PatchJSON(path, testkit.JSON{"name": "Acme Inc", "notes": "Key account"}).AssertOK()
		require.Len(t, updated.All(), 1)
		assert.Equal(t, []string{"seats", "plan"}, sortedPair(updated.All()[0].ChangedAttributeKeys))
		assert.Len(t, created.All(), 2)

		page := d.Owner.GetJSON("/api/v1/companies?limit=1").AssertOK().
			AssertJSONPath("data.0.name", "Globex")
		d.Owner.GetJSON("/api/v1/companies?limit=1&cursor="+page.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.0.name", "Acme Inc")
		d.Owner.GetJSON("/api/v1/companies?q=globe").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON("/api/v1/companies?cursor=forged").AssertStatus(400)
		d.Owner.GetJSON("/api/v1/companies?filter[nope]=1").AssertStatus(400)
	})

	t.Run("refuses bad input", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := company(t, d, testkit.JSON{"name": "Acme", "external_id": "org_1"})
		other := company(t, d, testkit.JSON{"name": "Globex"})

		d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"domain": "x.dev"}).
			AssertStatus(422).AssertJSONPath("code", "company_name_required")
		d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "A", "external_id": "org_1"}).
			AssertStatus(409).AssertJSONPath("code", "company_exists")
		d.Owner.PatchJSON(other, testkit.JSON{"external_id": "org_1"}).
			AssertStatus(409).AssertJSONPath("code", "company_exists")
		d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "A", "custom_attributes": testkit.JSON{"Bad": 1}}).
			AssertStatus(422).AssertJSONPath("code", "invalid_custom_attributes")
		d.Owner.PatchJSON(path, testkit.JSON{"custom_attributes": testkit.JSON{"Bad": 1}}).AssertStatus(422)
		d.Owner.PostJSON("/api/v1/companies?upsert=maybe", testkit.JSON{"name": "A"}).AssertStatus(400)
		d.Owner.PatchJSON(path, testkit.JSON{"name": "  "}).AssertOK().AssertJSONPath("data.name", "Acme")
	})

	t.Run("upserts by external id, then domain", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		upsert := "/api/v1/companies?upsert=true"

		first := d.Owner.PostJSON(upsert, testkit.JSON{"domain": "www.acme.dev"}).AssertCreated().
			AssertJSONPath("meta.upserted", "created").
			AssertJSONPath("data.name", "acme.dev")
		id := first.String("data.id")

		d.Owner.PostJSON(upsert, testkit.JSON{"domain": "ACME.dev", "name": "Acme", "external_id": "org_1"}).
			AssertOK().
			AssertJSONPath("meta.upserted", "updated").
			AssertJSONPath("data.id", id).
			AssertJSONPath("data.name", "Acme")
		d.Owner.PostJSON(upsert, testkit.JSON{
			"external_id": "org_1", "custom_attributes": testkit.JSON{"plan": "pro"},
		}).
			AssertOK().AssertJSONPath("data.id", id).AssertJSONPath("data.custom_attributes.plan", "pro")
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "org_2"}).AssertCreated().
			AssertJSONPath("data.name", "org_2")
		d.Owner.PostJSON(upsert, testkit.JSON{"name": "Nothing"}).
			AssertStatus(422).AssertJSONPath("code", "upsert_key_required")
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "org_3", "custom_attributes": testkit.JSON{"Bad": 1}}).
			AssertStatus(422)
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "org_1", "custom_attributes": testkit.JSON{"Bad": 1}}).
			AssertStatus(422)
	})

	t.Run("tags a company up to the limit and filters by tag", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := company(t, d, testkit.JSON{"name": "Acme"})
		company(t, d, testkit.JSON{"name": "Globex"})

		res := d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"enterprise"}}).AssertOK().
			AssertJSONPath("data.tags.0.name", "enterprise")
		id := res.String("data.tags.0.id")

		d.Owner.GetJSON("/api/v1/companies?filter[tag]="+id).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "Acme")
		d.Owner.GetJSON("/api/v1/companies?filter[tag]=bogus").AssertStatus(400)
		d.Owner.PatchJSON(path, testkit.JSON{"remove_tags": []string{id}}).AssertOK().AssertJSONPath("data.tags.#", 0)

		names := make([]string, 50)
		for i := range names {
			names[i] = fmt.Sprintf("tag-%d", i)
		}

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": names}).AssertOK()
		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"overflow"}}).
			AssertStatus(422).AssertJSONPath("code", "tag_limit_reached")
	})

	t.Run("deleting takes its own permission and leaves people without a company", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		deleted := tests.Record[appevents.CompanyDeleted](d.TC)
		member, _ := d.Join(t, enums.RoleMember)
		path := company(t, d, testkit.JSON{"name": "Acme"})
		id := path[len("/api/v1/companies/"):]
		person := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ann"}).AssertCreated().String("data.id")
		d.Owner.PatchJSON("/api/v1/contacts/"+person, testkit.JSON{"company_id": id}).AssertOK()

		member.GetJSON(path).AssertOK()
		member.PatchJSON(path, testkit.JSON{"notes": "fine"}).AssertOK()
		member.Delete(path).AssertForbidden()

		d.Owner.Delete(path).AssertNoContent()
		d.Owner.Delete(path).AssertNotFound()
		d.Owner.GetJSON(path).AssertNotFound()
		d.Owner.PatchJSON(path, testkit.JSON{"name": "Ghost"}).AssertNotFound()
		d.Owner.GetJSON(path + "/contacts").AssertNotFound()
		d.Owner.GetJSON(path + "/conversations").AssertNotFound()
		d.Owner.GetJSON("/api/v1/contacts/"+person).AssertOK().AssertJSONPath("data.company", nil)
		assert.Len(t, deleted.All(), 1)
	})

	t.Run("lists a company's people and conversations", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := company(t, d, testkit.JSON{"name": "Acme"})
		id := path[len("/api/v1/companies/"):]
		bob := d.Customer(t, "bob", "From Bob")
		d.TC.Clock.Travel(time.Minute)
		ann := d.Customer(t, "ann", "From Ann")
		d.Customer(t, "cal", "Elsewhere")

		for _, res := range []*testkit.Response{bob, ann} {
			path := "/api/v1/contacts/" + res.String("data.contact.id")
			d.Owner.PatchJSON(path, testkit.JSON{"company_id": id}).AssertOK()
		}

		people := d.Owner.GetJSON(path+"/contacts?limit=1").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON(path+"/contacts?cursor="+people.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.#", 1)
		d.Owner.GetJSON(path + "/contacts?cursor=forged").AssertStatus(400)

		d.Owner.GetJSON(path+"/conversations").AssertOK().
			AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.subject", "From Ann")
		d.Owner.GetJSON("/api/v1/companies/bogus/conversations").AssertNotFound()

		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[company]="+id).AssertOK().
			AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[company]=bogus").AssertStatus(400)
	})

	t.Run("another workspace's company is not found", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		path := company(t, other, testkit.JSON{"name": "Theirs"})

		d.Owner.GetJSON(path).AssertNotFound()
		d.Owner.PatchJSON(path, testkit.JSON{"name": "Mine"}).AssertNotFound()
		d.Owner.Delete(path).AssertNotFound()
		d.Owner.GetJSON("/api/v1/companies/bogus").AssertNotFound()
		d.Owner.PatchJSON("/api/v1/companies/bogus", testkit.JSON{"name": "x"}).AssertNotFound()
		d.Owner.Delete("/api/v1/companies/bogus").AssertNotFound()
		d.Owner.GetJSON("/api/v1/companies/bogus/contacts").AssertNotFound()
	})

	t.Run("enforces token abilities", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := company(t, d, testkit.JSON{"name": "Acme"})
		reader := d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read")

		reader.GetJSON(path).AssertOK()
		reader.GetJSON(path + "/contacts").AssertOK()
		reader.PatchJSON(path, testkit.JSON{"name": "x"}).AssertForbidden()
		reader.PostJSON("/api/v1/companies", testkit.JSON{"name": "x"}).AssertForbidden()
		reader.Delete(path).AssertForbidden()
		reader.GetJSON(path + "/conversations").AssertForbidden()
	})
}

/*
sortedPair returns the keys with the removed key first, as the assertion
reads them.
*/
func sortedPair(keys []string) []string {
	if len(keys) == 2 && keys[0] == "plan" {
		return []string{keys[1], keys[0]}
	}

	return keys
}
