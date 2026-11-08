package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
published creates and publishes an article, returning its id.
*/
func (c center) published(t *testing.T, body testkit.JSON) string {
	t.Helper()
	id := c.article(t, body).String("data.id")
	c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()

	return id
}

/*
audience saves an audience of the rule and returns its id.
*/
func (c center) audience(t *testing.T, name string, rules testkit.JSON) string {
	t.Helper()

	return c.d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": name, "rules": rules}).
		AssertCreated().String("data.id")
}

/*
titles searches dimmi's knowledge as a reader in the allowed audiences.
*/
func (c center) titles(t *testing.T, query string, allowed ...string) []string {
	t.Helper()
	opts := contracts.KnowledgeSearchOptions{
		Visibility: []contracts.KnowledgeVisibility{"public"}, AudienceFiltered: true,
	}
	for _, public := range allowed {
		id, err := c.d.TC.App.IDs.Decode(string(models.PrefixAudience), public)
		require.NoError(t, err)
		opts.AllowedAudienceIDs = append(opts.AllowedAudienceIDs, id)
	}

	res, err := c.d.TC.App.Knowledge.Search(t.Context(), c.d.TC.Workspace.Workspace.ID, query, opts)
	require.NoError(t, err)

	out := []string{}
	for _, ch := range res.Chunks {
		out = append(out, ch.Title)
	}

	return out
}

func TestArticleAudiences(t *testing.T) {
	t.Parallel()

	t.Run("keeps an article from readers outside its audience everywhere they read", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		users := c.audience(t, "Signed-in users", testkit.JSON{"visitors": "identified"})
		id := c.published(t, testkit.JSON{"title": "Enterprise billing", "body_markdown": "Invoices go out monthly."})
		c.page("/articles/enterprise-billing").AssertOK()

		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": users}).AssertOK().
			AssertJSONPath("data.audience_id", users)
		c.page("/articles/enterprise-billing").AssertNotFound()
		c.page("/articles/enterprise-billing.md").AssertNotFound()
		assert.NotContains(t, string(c.page("/search?q=invoices").AssertOK().Body), "Enterprise billing")
		assert.NotContains(t, string(c.page("").AssertOK().Body), "Enterprise billing")

		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": nil}).AssertOK()
		c.page("/articles/enterprise-billing").AssertOK()
	})

	t.Run("keeps everything in a collection to the collection's audience", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		users := c.audience(t, "Signed-in users", testkit.JSON{"visitors": "identified"})
		col := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Admins"}).
			AssertCreated().String("data.id")
		c.published(t, testkit.JSON{"title": "Rotate keys", "collection_id": col, "body_markdown": "Rotate often."})

		c.d.Owner.PatchJSON("/api/v1/collections/"+col+"/audience", testkit.JSON{"audience_id": users}).AssertOK().
			AssertJSONPath("data.audience_id", users)
		c.page("/articles/rotate-keys").AssertNotFound()
		assert.NotContains(t, string(c.page("").AssertOK().Body), "Admins")
	})

	t.Run("answers people only from articles they could read", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		pro := c.audience(t, "Pro", testkit.JSON{"rules": testkit.JSON{
			"field": "contact.custom.plan", "op": "eq", "value": "pro",
		}})
		open := c.published(t, testkit.JSON{"title": "Refund basics", "body_markdown": "Refunds take five days."})
		kept := c.published(t, testkit.JSON{"title": "Priority refunds", "body_markdown": "Pro refunds take one day."})
		c.d.Owner.PatchJSON("/api/v1/articles/"+kept+"/audience", testkit.JSON{"audience_id": pro}).AssertOK()
		assert.NotEmpty(t, open)

		assert.Contains(t, c.titles(t, "how long do refunds take", pro), "Priority refunds")
		assert.NotContains(t, c.titles(t, "how long do refunds take"), "Priority refunds")
		assert.Contains(t, c.titles(t, "how long do refunds take"), "Refund basics")
	})

	t.Run("refuses an audience a browser could satisfy, and one that does not exist", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		email := c.audience(t, "Gmail", testkit.JSON{"rules": testkit.JSON{
			"field": "contact.email_domain", "op": "eq", "value": "gmail.com",
		}})
		id := c.published(t, testkit.JSON{"title": "Anything", "body_markdown": "Text."})

		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": email}).
			AssertStatus(422).AssertJSONPath("code", "audience_unfit")
		ghost := models.PublicID(models.PrefixAudience, ids.New())
		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": ghost}).
			AssertStatus(422)
	})

	t.Run("refuses to delete an audience an article is kept to", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		users := c.audience(t, "Signed-in users", testkit.JSON{"visitors": "identified"})
		id := c.published(t, testkit.JSON{"title": "Members only", "body_markdown": "Text."})
		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": users}).AssertOK()

		c.d.Owner.GetJSON("/api/v1/audiences/"+users).AssertOK().AssertJSONPath("data.used_by.0.object", "article")
		c.d.Owner.Delete("/api/v1/audiences/"+users).AssertStatus(409).AssertJSONPath("used_by.0", "Members only")
	})
}
