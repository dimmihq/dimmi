package helpcenters_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestCollectionTree(t *testing.T) {
	t.Parallel()

	t.Run("nests three levels deep and no deeper, either way round", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		collection := func(name string) string {
			return c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": name}).
				AssertCreated().String("data.id")
		}
		move := func(id, parent string) *testkit.Response {
			return c.d.Owner.PatchJSON("/api/v1/collections/"+id, testkit.JSON{"parent_id": parent})
		}
		basics, billing, invoices, cards := collection("Basics"), collection("Billing"), collection("Invoices"),
			collection("Cards")

		move(invoices, billing).AssertOK().AssertJSONPath("data.parent_id", billing)
		move(cards, invoices).AssertOK().AssertJSONPath("data.parent_id", invoices)
		move(basics, cards).AssertStatus(422).AssertJSONPath("code", "collection_too_deep")
		move(billing, basics).AssertStatus(422)
		move(billing, cards).AssertStatus(422)
		move(billing, billing).AssertStatus(422)
		late := testkit.JSON{"name": "Late", "parent_id": cards}
		c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", late).AssertStatus(422)
		move(invoices, "").AssertOK().AssertJSONPath("data.parent_id", nil)
		move(invoices, basics).AssertOK()
	})

	t.Run("orders collections by the positions given", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		first := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "First"}).
			AssertCreated().String("data.id")
		c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Second"}).AssertCreated()
		c.d.Owner.PatchJSON("/api/v1/collections/"+first, testkit.JSON{"position": 5000}).AssertOK()

		c.d.Owner.GetJSON("/api/v1/help-centers/"+c.id+"/collections").AssertOK().
			AssertJSONPath("data.0.name", "Second").AssertJSONPath("data.1.name", "First")
	})
}
