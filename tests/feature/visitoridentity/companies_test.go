package visitoridentity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentityCompanies(t *testing.T) {
	t.Parallel()

	t.Run("adds every company the token lists, keeping the company claim the default", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()

		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub":     "user_1",
			"company": map[string]any{"id": "org_1", "name": "Acme"},
			"companies": []any{
				map[string]any{"id": "org_2", "name": "Globex"},
				map[string]any{"name": "No id"},
				map[string]any{"id": "org_1"},
			},
		}), nil).AssertCreated()
		path := "/api/v1/contacts/" + res.String("data.contact.id")

		d.owner.GetJSON(path).AssertOK().
			AssertJSONPath("data.company.name", "Acme").
			AssertJSONPath("data.companies.#", 2).
			AssertJSONPath("data.companies.1.name", "Globex")
	})

	t.Run("makes the first listed company the default when there is no company claim", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()

		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_1",
			"companies": []any{
				map[string]any{"id": "org_2", "name": "Globex"}, map[string]any{"id": "org_3", "name": "Initech"},
			},
		}), nil).AssertCreated()
		c := d.contact(res.String("data.contact.id"))

		assert.NotNil(t, c.CompanyID)
		d.owner.GetJSON("/api/v1/contacts/"+res.String("data.contact.id")).AssertOK().
			AssertJSONPath("data.company.name", "Globex").
			AssertJSONPath("data.companies.#", 2)
	})
}
