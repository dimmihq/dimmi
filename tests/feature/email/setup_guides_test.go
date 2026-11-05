package email_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/dns"
)

func TestSetupGuides(t *testing.T) {
	t.Parallel()

	t.Run("opens a public mail service's address on that service's steps", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		res := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{"address": "acme.help@gmail.com"}).
			AssertCreated()

		forwarding := res.String("data.forwarding_address")
		res.AssertJSONPath("data.setup_guides.0.key", "gmail")
		res.AssertJSONPath("data.setup_guides.0.provider", "gmail")
		res.AssertJSONPath("data.setup_guides.0.recommended", true)
		res.AssertJSONPath("data.setup_guides.1.recommended", false)
		res.AssertJSONPath("data.setup_guides.0.deep_link", "https://mail.google.com/mail/u/0/#settings/fwdandpop")
		res.AssertJSONPath("data.setup_guides.#", 12.0)
		assert.Contains(t, res.String("data.setup_guides.0.steps.1"), forwarding)
		assert.Contains(t, res.String("data.setup_guides.0.steps.4"), "Test forwarding")
		assert.Contains(t, res.String("data.setup_guides.0.warnings.0"), "Sent folder")
	})

	t.Run("knows the other public mail services by their domains", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		owner := tc.ActingAsOwner()

		for domain, want := range map[string]string{
			"hotmail.com": "outlook_com", "icloud.com": "icloud", "yahoo.com": "yahoo",
		} {
			owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "acme@" + domain}).AssertCreated().
				AssertJSONPath("data.setup_guides.0.key", want)
		}
	})

	t.Run("reads the mail service of a company domain from its mail exchangers", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		owner := tc.ActingAsOwner()

		cases := map[string]string{
			"workspace.test": "aspmx.l.google.com.", "m365.test": "m365-test.mail.protection.outlook.com.",
			"fast.test": "in1-smtp.messagingengine.fastmail.com.", "zoho.test": "mx.zoho.eu.",
			"routed.test": "route1.mx.cloudflare.net.", "own.test": "mail.own.test.",
		}
		want := map[string]string{
			"workspace.test": "google_workspace", "m365.test": "microsoft_365", "fast.test": "fastmail",
			"zoho.test": "zoho", "routed.test": "cloudflare_routing", "own.test": "generic",
		}

		for domain, host := range cases {
			zone.Set(domain, dns.Records{MX: []dns.MX{{Host: host, Pref: 10}}})

			res := owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@" + domain}).
				AssertCreated()
			res.AssertJSONPath("data.setup_guides.0.key", want[domain])
			res.AssertJSONPath("data.setup_guides.0.recommended", true)
		}

		res := owner.GetJSON("/api/v1/email-addresses").AssertOK()
		assert.Equal(t, "generic", res.String("data.0.setup_guides.0.key"))
	})

	t.Run("fills the domain into the cloudflare link and keeps warnings plain", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		zone.Set("routed.test", dns.Records{MX: []dns.MX{{Host: "route1.mx.cloudflare.net.", Pref: 10}}})

		res := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@routed.test"}).
			AssertCreated()

		res.AssertJSONPath("data.setup_guides.0.deep_link", "https://dash.cloudflare.com/?to=/:account/routed.test/email")
		assert.Contains(t, res.String("data.setup_guides.0.steps.0"), "routed.test")
		assert.NotContains(t, res.String("data.setup_guides.0.steps.0"), "email_setup.")
	})

	t.Run("remembers a domain's mail service for an hour", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		owner := tc.ActingAsOwner()
		zone.Set("moving.test", dns.Records{MX: []dns.MX{{Host: "aspmx.l.google.com.", Pref: 10}}})

		created := owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@moving.test"}).
			AssertCreated()
		created.AssertJSONPath("data.setup_guides.0.key", "google_workspace")

		zone.Set("moving.test", dns.Records{MX: []dns.MX{{Host: "x.mail.protection.outlook.com.", Pref: 10}}})

		owner.GetJSON("/api/v1/email-addresses/"+created.String("data.id")).AssertOK().
			AssertJSONPath("data.setup_guides.0.key", "google_workspace")
	})

	t.Run("falls back to the generic steps for a domain without mail exchangers", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@nomail.test"}).
			AssertCreated().AssertJSONPath("data.setup_guides.0.key", "generic")
	})

	t.Run("lists no guide twice", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		res := tc.ActingAsOwner().GetJSON("/api/v1/email-addresses").AssertOK()

		seen := map[string]bool{}
		for i := range 12 {
			key := res.String("data.0.setup_guides." + strconv.Itoa(i) + ".key")
			assert.False(t, seen[key], key)
			seen[key] = true
		}
	})
}
