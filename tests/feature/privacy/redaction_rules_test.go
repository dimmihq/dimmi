package privacy_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

const rules = "/api/v1/redaction-rules"

func TestRedactionRules(t *testing.T) {
	t.Parallel()

	t.Run("starts with card numbers on and social security numbers off", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON(rules).AssertOK().
			AssertJSONPath("data.rules.#", 0).
			AssertJSONPath("data.built_ins.card_numbers", true).
			AssertJSONPath("data.built_ins.us_ssn", false)
	})

	t.Run("adds, changes, reorders and removes rules and keeps each in the audit log", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		created := tests.Record[appevents.RedactionRuleCreated](d.TC)
		updated := tests.Record[appevents.RedactionRuleUpdated](d.TC)
		deleted := tests.Record[appevents.RedactionRuleDeleted](d.TC)

		first := d.Owner.PostJSON(rules, testkit.JSON{"name": "Accounts", "pattern": `ACCT-\d+`}).AssertCreated().
			AssertJSONPath("data.object", "redaction_rule").
			AssertJSONPath("data.replacement", "[redacted]").
			AssertJSONPath("data.enabled", true).
			AssertJSONPath("data.position", 0).String("data.id")
		second := d.Owner.PostJSON(rules, testkit.JSON{
			"name": "Passwords", "pattern": `(?i)password:\s*\S+`, "replacement": "password: [hidden]",
			"enabled": false,
		}).AssertCreated().AssertJSONPath("data.position", 1).String("data.id")

		d.Owner.PatchJSON(rules+"/"+first, testkit.JSON{"name": " Account ids ", "enabled": false}).AssertOK().
			AssertJSONPath("data.name", "Account ids").AssertJSONPath("data.enabled", false)
		d.Owner.PatchJSON(rules+"/"+first, testkit.JSON{"name": "Account ids"}).AssertOK()
		require.Len(t, updated.All(), 1)
		assert.Equal(t, []string{"name", "enabled"}, updated.All()[0].Changes)

		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{second, first}, "us_ssn": true}).AssertOK().
			AssertJSONPath("data.rules.0.id", second).
			AssertJSONPath("data.rules.1.id", first).
			AssertJSONPath("data.built_ins.us_ssn", true)
		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{second, first}, "us_ssn": true}).AssertOK()

		d.Owner.Delete(rules + "/" + first).AssertNoContent()
		d.Owner.Delete(rules + "/" + first).AssertNotFound()
		d.Owner.PatchJSON(rules+"/"+first, testkit.JSON{"name": "x"}).AssertNotFound()
		d.Owner.GetJSON(rules).AssertOK().AssertJSONPath("data.rules.#", 1)
		assert.Len(t, created.All(), 2)
		assert.Len(t, deleted.All(), 1)

		audit := d.TC.Table("audit_logs")
		audit.AssertCount(2, testkit.Row{"event": "redaction_rule.created"})
		audit.AssertCount(1, testkit.Row{"event": "redaction_rule.updated"})
		audit.AssertCount(1, testkit.Row{"event": "redaction_rule.deleted"})
		audit.AssertCount(1, testkit.Row{"event": "redaction_settings.updated"})
	})

	t.Run("refuses a pattern that does not compile or is too long", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON(rules, testkit.JSON{"name": "Broken", "pattern": "(unclosed"}).
			AssertStatus(422).AssertJSONPath("code", "invalid_pattern")
		d.Owner.PostJSON(rules, testkit.JSON{"name": "Long", "pattern": fmt.Sprintf("%0257d", 1)}).
			AssertValidationError("pattern")

		id := d.Owner.PostJSON(rules, testkit.JSON{"name": "Fine", "pattern": "x+"}).AssertCreated().String("data.id")
		d.Owner.PatchJSON(rules+"/"+id, testkit.JSON{"pattern": "[z"}).
			AssertStatus(422).AssertJSONPath("code", "invalid_pattern")
	})

	t.Run("keeps at most ten rules", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		for i := range 10 {
			d.Owner.PostJSON(rules, testkit.JSON{"name": fmt.Sprint("Rule ", i), "pattern": "x"}).AssertCreated()
		}

		d.Owner.PostJSON(rules, testkit.JSON{"name": "Eleventh", "pattern": "x"}).
			AssertStatus(422).AssertJSONPath("code", "redaction_rule_limit_reached")
	})

	t.Run("refuses an order that does not name each rule once", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		a := d.Owner.PostJSON(rules, testkit.JSON{"name": "A", "pattern": "a"}).AssertCreated().String("data.id")
		d.Owner.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertCreated()

		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{a}}).
			AssertStatus(422).AssertJSONPath("code", "invalid_order")
		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{a, "rxr_bogus"}}).
			AssertStatus(422).AssertJSONPath("code", "invalid_order")
		d.Owner.PatchJSON(rules+"/rxr_bogus", testkit.JSON{"name": "x"}).AssertNotFound()
		d.Owner.Delete(rules + "/rxr_bogus").AssertNotFound()
	})

	t.Run("lets only those who manage redaction change it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := d.Owner.PostJSON(rules, testkit.JSON{"name": "A", "pattern": "a"}).AssertCreated().String("data.id")
		member, _ := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)

		member.GetJSON(rules).AssertForbidden()
		member.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertForbidden()
		member.PostJSON("/api/v1/redaction-tests", testkit.JSON{"text": "x"}).AssertForbidden()
		admin.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertCreated()
		admin.PatchJSON(rules+"/"+id, testkit.JSON{"enabled": false}).AssertOK()
	})
}
