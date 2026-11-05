package privacy_test

import (
	"fmt"
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestLinkSafety(t *testing.T) {
	t.Parallel()

	t.Run("starts with the checks on and no domains listed", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON("/api/v1/workspace").AssertOK().
			AssertJSONPath("data.link_safety.enabled", true).
			AssertJSONPath("data.link_safety.allow_domains.#", 0).
			AssertJSONPath("data.link_safety.block_domains.#", 0).
			AssertJSONPath("data.link_safety.strip_inbound_email_links", false)
	})

	t.Run("saves domains in their canonical form and keeps what was not sent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{
			"allow_domains": []string{"Docs.Example.com.", "docs.example.com"},
			"block_domains": []string{"Café.example"}, "strip_inbound_email_links": true,
		}}).AssertOK().
			AssertJSONPath("data.link_safety.allow_domains.#", 1).
			AssertJSONPath("data.link_safety.allow_domains.0", "docs.example.com").
			AssertJSONPath("data.link_safety.block_domains.0", "xn--caf-dma.example").
			AssertJSONPath("data.link_safety.strip_inbound_email_links", true)

		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"enabled": false}}).AssertOK().
			AssertJSONPath("data.link_safety.enabled", false).
			AssertJSONPath("data.link_safety.allow_domains.0", "docs.example.com")
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"enabled": false}}).AssertOK()
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"allow_domains": []string{}}}).
			AssertOK().AssertJSONPath("data.link_safety.allow_domains.#", 0)

		d.TC.Table("audit_logs").AssertCount(3, testkit.Row{"event": "workspace.updated"})
	})

	t.Run("refuses a public suffix, an address and a list too long", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		many := make([]string, 0, 501)
		for i := range 501 {
			many = append(many, fmt.Sprintf("site%d.example", i))
		}

		for _, list := range [][]string{{"co.uk"}, {"10.0.0.1"}, {"https://evil.example/x"}} {
			d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"block_domains": list}}).
				AssertStatus(422).AssertJSONPath("code", "invalid_link_domains")
		}

		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"block_domains": many}}).
			AssertValidationError("link_safety.block_domains")
	})

	t.Run("lets every member read the checks and only admins change them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.link_safety.enabled", true)
		member.PatchJSON("/api/v1/workspace", testkit.JSON{"link_safety": testkit.JSON{"enabled": false}}).
			AssertForbidden()
		d.Owner.PatchJSON("/api/v1/workspaces/"+d.TC.Workspace.Workspace.ID.String(),
			testkit.JSON{"trashed": false, "link_safety": testkit.JSON{}}).AssertValidationError("link_safety")
	})
}
