package privacy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/tests"
)

func TestRedactionRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses a membership that may not read or manage redaction", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acts := d.TC.App.Actions.Inbox
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"settings:write"})
		ctx := t.Context()

		_, err := acts.ListRedactionRules.Handle(ctx, redaction.ListRulesInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.UpdateRedactionSettings.Handle(ctx, redaction.UpdateSettingsInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.CreateRedactionRule.Handle(ctx, redaction.CreateRuleInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.UpdateRedactionRule.Handle(ctx, redaction.UpdateRuleInput{Membership: none, RuleID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		err = acts.DeleteRedactionRule.Handle(ctx, redaction.DeleteRuleInput{Membership: none, RuleID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.TryRedaction.Handle(ctx, redaction.TryRedactionInput{Membership: none, Text: "x"})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})

	t.Run("refuses an order that names one rule twice", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		a := d.Owner.PostJSON(rules, testkit.JSON{"name": "A", "pattern": "a"}).AssertCreated().String("data.id")
		d.Owner.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertCreated()

		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{a, a}}).
			AssertStatus(422).AssertJSONPath("code", "invalid_order")
	})

	t.Run("shows no redactions for a message whose record of them is unreadable", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := start(t, d, "Hi", "Hello")
		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE messages SET metadata = '{"redactions": "x"}'`)
		require.NoError(t, err)

		d.Owner.GetJSON(path + "/messages").AssertOK().AssertJSONMissing("data.0.redactions")
	})
}

func TestRedactionFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails every rule request when the rules cannot be read", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := d.Owner.PostJSON(rules, testkit.JSON{"name": "A", "pattern": "a"}).AssertCreated().String("data.id")
		path := start(t, d, "Hi", "Hello")
		d.TC.BreakTable("redaction_rules")

		d.Owner.GetJSON(rules).AssertStatus(500)
		d.Owner.PatchJSON(rules, testkit.JSON{"us_ssn": true}).AssertStatus(500)
		d.Owner.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertStatus(500)
		d.Owner.PatchJSON(rules+"/"+id, testkit.JSON{"name": "C"}).AssertStatus(500)
		d.Owner.Delete(rules + "/" + id).AssertStatus(500)
		d.Owner.PostJSON("/api/v1/redaction-tests", testkit.JSON{"text": "x"}).AssertStatus(500)
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "x"}).AssertStatus(500)
	})

	t.Run("fails rule writes the database refuses", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		a := d.Owner.PostJSON(rules, testkit.JSON{"name": "A", "pattern": "a"}).AssertCreated().String("data.id")
		b := d.Owner.PostJSON(rules, testkit.JSON{"name": "B", "pattern": "b"}).AssertCreated().String("data.id")
		d.TC.FailWrites("redaction_rules", "INSERT", "UPDATE", "DELETE")
		d.TC.FailWrites("workspaces", "UPDATE")

		d.Owner.PostJSON(rules, testkit.JSON{"name": "C", "pattern": "c"}).AssertStatus(500)
		d.Owner.PatchJSON(rules+"/"+a, testkit.JSON{"name": "C"}).AssertStatus(500)
		d.Owner.Delete(rules + "/" + a).AssertStatus(500)
		d.Owner.PatchJSON(rules, testkit.JSON{"order": []string{b, a}}).AssertStatus(500)
		d.Owner.PatchJSON(rules, testkit.JSON{"us_ssn": true}).AssertStatus(500)
	})

	t.Run("fails writes while the workspace's redaction cannot be used", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := start(t, d, "Hi", "Hello")
		db := d.TC.Operator()

		bad := `INSERT INTO redaction_rules (workspace_id, name, pattern) VALUES ($1, 'Bad', '(')`
		_, err := db.Exec(t.Context(), bad, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "x"}).AssertStatus(500)

		_, err = db.Exec(t.Context(), `DELETE FROM redaction_rules`)
		require.NoError(t, err)
		_, err = db.Exec(t.Context(), `UPDATE workspaces SET settings = jsonb_set(settings, '{redaction}', '"x"')`)
		require.NoError(t, err)
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "x"}).AssertStatus(500)
		d.Owner.PatchJSON(path, testkit.JSON{"subject": "x"}).AssertStatus(500)
		d.Owner.GetJSON(rules).AssertStatus(500)
		d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.InboxID(), "author": "contact", "subject": "Hi", "body": "Hello",
			"contact": testkit.JSON{"email": "bob@example.com"},
		}).AssertStatus(500)
	})
}
