package helpcenters_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
reader signs a customer in to the help center and returns their cookie.
*/
func (c center) reader(t *testing.T, sub, email string) *fhttp.Cookie {
	t.Helper()
	c.customer(t, sub, email)

	return c.readerCookie(t, email)
}

/*
typeID returns the public id of the ticket type named.
*/
func (c center) typeID(t *testing.T, name string) string {
	t.Helper()

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	c.d.Owner.GetJSON("/api/v1/ticket-types").AssertOK().Decode(&body)

	for _, ty := range body.Data {
		if ty.Name == name {
			return ty.ID
		}
	}

	t.Fatalf("no ticket type %s", name)

	return ""
}

/*
post sends a form of the portal as the reader.
*/
func (c center) post(cookie *fhttp.Cookie, path string, values url.Values) *testkit.Response {
	return c.d.TC.Client().WithCookie(cookie).WithHeader("Content-Type", "application/x-www-form-urlencoded").
		WithHeader("Accept", "text/html").Send("POST", c.address(path), values.Encode())
}

func TestPortalRequests(t *testing.T) {
	t.Parallel()

	t.Run("sends a reader who is not signed in to sign in first", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)

		res := c.page("/tickets").AssertStatus(fhttp.StatusSeeOther)
		assert.Contains(t, res.Header.Get("Location"), "sign-in")
	})

	t.Run("opens a request from the form, lists it and takes a reply in its thread", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		cookie := c.reader(t, "user_ada", "ada@example.com")
		bug := c.typeID(t, "Bug report")

		picker := string(c.as(cookie, "/tickets/new").AssertOK().Body)
		assert.Contains(t, picker, "Bug report")
		assert.Contains(t, picker, "Question")
		form := string(c.as(cookie, "/tickets/new/"+bug).AssertOK().Body)
		assert.Contains(t, form, "Steps to reproduce")
		assert.NotContains(t, form, "Severity")

		refused := c.post(cookie, "/tickets/new/"+bug, url.Values{"title": {""}, "key": {"k1"}})
		refused.AssertStatus(fhttp.StatusUnprocessableEntity)
		c.d.TC.Table("conversations").AssertCount(0, testkit.Row{})

		sent := url.Values{
			"title": {"Export is broken"}, "description": {"The CSV is empty"}, "key": {"k2"},
			"field.steps": {"Open reports"},
		}
		made := c.post(cookie, "/tickets/new/"+bug, sent).AssertStatus(fhttp.StatusSeeOther)
		assert.Equal(t, c.local("/tickets/T-1"), made.Header.Get("Location"))
		c.post(cookie, "/tickets/new/"+bug, sent).AssertStatus(fhttp.StatusSeeOther)
		c.d.TC.Table("tickets").AssertCount(1, testkit.Row{})

		list := string(c.as(cookie, "/tickets").AssertOK().Body)
		assert.Contains(t, list, "Export is broken")
		assert.Contains(t, list, "T-1")

		page := string(c.as(cookie, "/tickets/T-1").AssertOK().Body)
		assert.Contains(t, page, "The CSV is empty")
		assert.Contains(t, page, "Open reports")
		assert.Contains(t, page, "We&#39;re on it")

		c.post(cookie, "/tickets/T-1/messages", url.Values{"body": {"Any news?"}}).
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", c.local("/tickets/T-1"))
		assert.Contains(t, string(c.as(cookie, "/tickets/T-1").AssertOK().Body), "Any news?")
		c.d.TC.Table("messages").AssertCount(2, testkit.Row{"type": "message"})

		other := c.reader(t, "user_bo", "bo@example.com")
		c.as(other, "/tickets/T-1").AssertNotFound()
	})

	t.Run("leads the old conversation pages to the requests", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		cookie := c.reader(t, "user_cy", "cy@example.com")

		c.as(cookie, "/conversations").AssertStatus(fhttp.StatusSeeOther).
			AssertHeader("Location", c.local("/tickets"))
	})
}
