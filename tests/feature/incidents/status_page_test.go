package incidents_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
page fetches a public help center page as a browser would, signed out.
*/
func (d *desk) page(t *testing.T, path string) string {
	t.Helper()

	res := d.TC.Client().WithHeader("Accept", "text/html").Send(fhttp.MethodGet, path, nil)
	require.Equal(t, fhttp.StatusOK, res.Status, string(res.Body))

	return string(res.Body)
}

func TestStatusPage(t *testing.T) {
	t.Parallel()

	t.Run("shows every area operational and links itself from the header", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		center := d.Owner.PostJSON("/api/v1/help-centers", testkit.JSON{"name": "Acme Help"}).AssertCreated().
			String("data.subdomain")

		body := d.page(t, "/hc/"+center+"/status")
		assert.Contains(t, body, "All systems operational")
		assert.Contains(t, body, `data-component="Sign-in"`)
		assert.Contains(t, body, "No incidents right now.")
		assert.Contains(t, body, `href="/hc/`+center+`/status"`)
		assert.Contains(t, d.page(t, "/hc/"+center), `href="/hc/`+center+`/status"`)
	})

	t.Run("shows the worst active severity per area, the update log and the history", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		center := d.Owner.PostJSON("/api/v1/help-centers", testkit.JSON{"name": "Acme Help"}).AssertCreated().
			String("data.subdomain")
		incident := func(title, severity string, areas ...string) string {
			return d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
				"type_id": d.types["Incident"], "title": title,
				"fields": testkit.JSON{"severity": severity, "areas": areas},
			}).AssertCreated().String("data.id")
		}
		signIn := incident("Sign-in outage", "critical", "Sign-in")
		incident("Slow payments", "minor", "Payments", "Sign-in")
		old := incident("Email delays", "major", "Email")
		d.Owner.PostJSON("/api/v1/tickets/"+signIn+"/public-updates", testkit.JSON{"body": "Fix is rolling out."}).
			AssertCreated()
		d.Owner.PatchJSON("/api/v1/tickets/"+old, testkit.JSON{"state_id": d.state(t, "Resolved")}).AssertOK()

		body := d.page(t, "/hc/"+center+"/status")
		assert.Contains(t, body, `data-overall="major_outage"`)
		assert.Contains(t, body, "Sign-in outage")
		assert.Contains(t, body, "Fix is rolling out.")
		assert.Contains(t, body, "Slow payments")
		assert.Contains(t, body, "Email delays")
		pill := `"[^>]*>\s*<span[^>]*>[^<]*</span>\s*<span[^>]*>\s*<span[^>]*></span>\s*`
		assert.Regexp(t, `data-component="Sign-in`+pill+`Major outage`, body)
		assert.Regexp(t, `data-component="Payments`+pill+`Degraded performance`, body)
		assert.Regexp(t, `data-component="Email`+pill+`Operational`, body)
	})
}

/*
state returns the id of the ticket state named.
*/
func (d *desk) state(t *testing.T, name string) string {
	t.Helper()

	res := d.Owner.GetJSON("/api/v1/ticket-states").AssertOK()
	for i := range num(res) {
		if res.String("data."+itoa(i)+".name") == name {
			return res.String("data." + itoa(i) + ".id")
		}
	}

	t.Fatalf("no state %s", name)

	return ""
}
