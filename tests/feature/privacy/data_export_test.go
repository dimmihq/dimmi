package privacy_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const dataExports = "/api/v1/data-exports"

/*
exportAll asks for an export of every conversation the client sees, in
any status, builds it and returns the export once it is ready.
*/
func exportAll(t *testing.T, d tests.Desk, client *testkit.Client, body testkit.JSON) *testkit.Response {
	t.Helper()

	if body["selection"] == nil {
		body["selection"] = testkit.JSON{"filter": testkit.JSON{"view": "all", "status": "any"}}
	}

	if body["kind"] == nil {
		body["kind"] = "conversations"
	}

	id := client.PostJSON(dataExports, body).AssertStatus(202).
		AssertJSONPath("data.object", "data_export").
		AssertJSONPath("data.status", "pending").String("data.id")
	d.TC.Drain()

	return client.GetJSON(dataExports + "/" + id).AssertOK()
}

/*
downloaded returns the file of a ready export.
*/
func downloaded(t *testing.T, client *testkit.Client, export *testkit.Response) *testkit.Response {
	t.Helper()

	export.AssertJSONPath("data.status", "ready")

	return client.GetJSON(export.String("data.url")).AssertOK()
}

func TestDataExports(t *testing.T) {
	t.Parallel()

	t.Run("writes each conversation's transcript and zips several", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ready := tests.Record[appevents.DataExportReady](d.TC)
		first := d.Customer(t, "ann", "Order").String("data.id")
		d.Customer(t, "bob", "Refund")

		export := exportAll(t, d, d.Owner, testkit.JSON{"format": "txt"}).
			AssertJSONPath("data.row_count", 2).AssertJSONPath("data.file_name", "export.zip")
		files := unzip(t, downloaded(t, d.Owner, export).AssertHeader("Content-Type", "application/zip").Body)

		require.Len(t, files, 2)
		assert.Contains(t, files["conversation-1.txt"], "Conversation #1: Order")
		assert.Contains(t, files["conversation-1.txt"], "Customer: ann <ann@example.com>")
		assert.Contains(t, files["conversation-1.txt"], "ann, ")
		assert.Contains(t, files["conversation-1.txt"], "Hello from ann")
		require.Len(t, ready.All(), 1)
		assert.Equal(t, 2, ready.All()[0].Rows)

		picked := testkit.JSON{"ids": []string{first}}
		single := exportAll(t, d, d.Owner, testkit.JSON{"format": "txt", "selection": picked})
		body := downloaded(t, d.Owner, single).AssertHeader("Content-Type", "text/plain; charset=utf-8").
			AssertHeader("Content-Disposition", `attachment; filename="conversation-1.txt"`)
		assert.Contains(t, string(body.Body), "Hello from ann")
		assert.NotContains(t, string(body.Body), "Hello from bob")
	})

	t.Run("writes one spreadsheet row per conversation and defuses formulas", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "=HYPERLINK(1)")

		sheet := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"})).
			AssertHeader("Content-Type", "text/csv; charset=utf-8").Body)

		lines := strings.Split(strings.TrimSpace(sheet), "\n")
		require.Len(t, lines, 2)
		assert.True(t, strings.HasPrefix(lines[0], "number,subject,status,contact,email"))
		assert.Contains(t, lines[1], "'=HYPERLINK(1)")
		assert.Contains(t, lines[1], "ann@example.com")
	})

	t.Run("writes the data with notes only when asked and masks what came before a rule", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PatchJSON(rules, testkit.JSON{"card_numbers": false}).AssertOK()
		path := start(t, d, "Card "+card, "My card is "+card)
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "VIP, be gentle"}).AssertCreated()
		d.Owner.PatchJSON(rules, testkit.JSON{"card_numbers": true}).AssertOK()

		plain := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{"format": "json"})).Body)
		noted := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{
			"format": "json", "include_notes": true,
		})).Body)

		assert.Contains(t, plain, "My card is •••• •••• •••• 4242")
		assert.Contains(t, plain, `"subject": "Card •••• •••• •••• 4242"`)
		assert.NotContains(t, plain, card)
		assert.NotContains(t, plain, "VIP, be gentle")
		assert.Contains(t, noted, "VIP, be gentle")
	})

	t.Run("exports customers by filter and by pick", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "Order")
		d.Customer(t, "bob", "Refund")
		bob := d.Owner.GetJSON("/api/v1/contacts?filter[email]=bob@example.com").AssertOK().String("data.0.id")

		sheet := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{
			"kind": "contacts", "format": "csv", "selection": testkit.JSON{"q": "ann"},
		})).Body)
		picked := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{
			"kind": "contacts", "format": "json", "selection": testkit.JSON{"ids": []string{bob}},
		})).Body)

		assert.Contains(t, sheet, "ann@example.com")
		assert.NotContains(t, sheet, "bob@example.com")
		assert.Contains(t, picked, `"email": "bob@example.com"`)
		assert.NotContains(t, picked, "ann@example.com")
	})

	t.Run("fills every column from who holds the conversation and whose it is", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := start(t, d, "Hello", "Hi")
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "Noted"}).AssertCreated()
		_, err := d.TC.Operator().Exec(t.Context(), `
			INSERT INTO companies (workspace_id, name) SELECT id, 'Acme' FROM workspaces;
			INSERT INTO teams (workspace_id, name) SELECT id, 'Billing' FROM workspaces;
			INSERT INTO tags (workspace_id, name) SELECT id, 'vip' FROM workspaces;
			UPDATE contacts SET company_id = (SELECT id FROM companies), email = NULL, custom_attributes = '{}';
			UPDATE conversations SET assignee_team_id = (SELECT id FROM teams),
				assignee_member_id = (SELECT id FROM workspace_members LIMIT 1);
			INSERT INTO conversation_tag (workspace_id, conversation_id, tag_id, created_by_type)
				SELECT workspace_id, id, (SELECT id FROM tags), 'system' FROM conversations;
			INSERT INTO contact_tag (workspace_id, contact_id, tag_id, created_by_type)
				SELECT workspace_id, id, (SELECT id FROM tags), 'system' FROM contacts;
			INSERT INTO messages (workspace_id, conversation_id, type, author_type, body)
				SELECT workspace_id, id, 'message', 'agent', 'Automatic hello' FROM conversations;
			UPDATE messages SET author_id = uuidv7() WHERE type = 'note'`)
		require.NoError(t, err)

		sheet := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"})).Body)
		text := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{
			"format": "txt", "include_notes": true,
		})).Body)
		people := string(downloaded(t, d.Owner, exportAll(t, d, d.Owner, testkit.JSON{
			"kind": "contacts", "format": "json", "selection": testkit.JSON{},
		})).Body)

		assert.Contains(t, sheet, "Acme")
		assert.Contains(t, sheet, "Billing")
		assert.Contains(t, sheet, "vip")
		assert.Contains(t, text, "Customer: Ann\n")
		assert.Contains(t, text, "System, ")
		assert.Contains(t, text, "Teammate (internal note), ")
		assert.Contains(t, people, `"company": "Acme"`)
		assert.Contains(t, people, `"vip"`)
		assert.Contains(t, people, `"custom_attributes": {}`)
	})

	t.Run("deletes an export before its file is built", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "csv"}).
			AssertStatus(202).String("data.id")

		d.Owner.Send("DELETE", dataExports+"/"+id, nil).AssertNoContent()
		d.TC.Drain()
		d.TC.Table("data_exports").AssertCount(0, testkit.Row{})
	})

	t.Run("lists the member's own exports and deletes one with its file", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		admin, _ := d.Join(t, enums.RoleAdmin)
		d.Customer(t, "ann", "Order")
		export := exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"})
		show := dataExports + "/" + export.String("data.id")

		d.Owner.GetJSON(dataExports).AssertOK().AssertJSONPath("data.0.id", export.String("data.id"))
		admin.GetJSON(dataExports).AssertOK().AssertJSONMissing("data.0")
		admin.GetJSON(show).AssertNotFound()
		admin.Send("DELETE", show, nil).AssertNotFound()
		d.Owner.Send("DELETE", show, nil).AssertNoContent()

		d.Owner.GetJSON(show).AssertNotFound()
		d.TC.Table("data_exports").AssertCount(0, testkit.Row{})
		d.Owner.GetJSON(dataExports + "/dex_nonsense").AssertNotFound()
	})
}

func TestDataExportScope(t *testing.T) {
	t.Parallel()

	t.Run("covers only what the member still sees when the file is built", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		admin, member := d.Join(t, enums.RoleAdmin)
		mine := d.Customer(t, "ann", "Mine").String("data.id")
		d.Customer(t, "bob", "Theirs")
		d.Owner.PatchJSON("/api/v1/conversations/"+mine, testkit.JSON{
			"assignee_id": models.PublicID(models.PrefixMember, member.ID),
		}).AssertOK()

		id := admin.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{"view": "all"}},
		}).AssertStatus(202).String("data.id")
		d.Owner.PatchJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, member.ID), testkit.JSON{
			"conversation_access": "assigned",
		}).AssertOK()
		d.TC.Drain()

		export := admin.GetJSON(dataExports+"/"+id).AssertOK().AssertJSONPath("data.row_count", 1)
		sheet := string(downloaded(t, admin, export).Body)
		assert.Contains(t, sheet, "Mine")
		assert.NotContains(t, sheet, "Theirs")
	})

	t.Run("fails the build for a member who no longer works in the workspace", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		admin, member := d.Join(t, enums.RoleAdmin)
		d.Customer(t, "ann", "Order")
		id := admin.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{"view": "all"}},
		}).AssertStatus(202).String("data.id")
		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE workspace_members SET status = 'deactivated' WHERE id = $1`, member.ID)
		require.NoError(t, err)

		d.TC.Drain()

		d.TC.Table("data_exports").AssertCount(1, testkit.Row{"status": "failed"})
		_, err = d.TC.Operator().Exec(t.Context(),
			`UPDATE workspace_members SET status = 'active' WHERE id = $1`, member.ID)
		require.NoError(t, err)
		admin.GetJSON(dataExports+"/"+id).AssertOK().AssertJSONPath("data.status", "failed").
			AssertJSONPath("data.error", "The export could not be built.")
	})
}

func TestDataExportRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses members without the export permission and notes without moderation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		member.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "csv"}).AssertForbidden()
		member.PostJSON(dataExports, testkit.JSON{"kind": "contacts", "format": "csv"}).AssertForbidden()
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "contacts", "format": "csv", "include_notes": true,
		}).AssertForbidden()
	})

	t.Run("refuses formats the kind does not come in and selections of the wrong rows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "contacts", "format": "txt"}).AssertStatus(422).
			AssertJSONPath("code", "invalid_export_format")
		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "doc"}).
			AssertValidationError("format")
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "csv", "selection": testkit.JSON{"ids": []string{"ctc_nonsense"}},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_selection")
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "contacts", "format": "csv", "selection": testkit.JSON{"ids": []string{"conv_nonsense"}},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_selection")
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{"view": "x"}},
		}).AssertStatus(400).AssertJSONPath("code", "invalid_filter")
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "contacts", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{"kind": "x"}},
		}).AssertStatus(400).AssertJSONPath("code", "invalid_filter")
	})

	t.Run("refuses more rows than the format allows with how many matched", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "Order")
		_, err := d.TC.Operator().Exec(t.Context(), `
			INSERT INTO conversations (workspace_id, number, inbox_id, channel_connection_id, contact_id, status)
			SELECT workspace_id, number + g, inbox_id, channel_connection_id, contact_id, status
			FROM conversations, generate_series(1, 1000) AS g`)
		require.NoError(t, err)

		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "txt", "selection": testkit.JSON{"filter": testkit.JSON{"view": "all"}},
		}).AssertStatus(422).AssertJSONPath("code", "export_too_large").
			AssertJSONPath("matched", 1001).AssertJSONPath("limit", 1000)
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "conversations", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{"view": "all"}},
		}).AssertStatus(202)
	})

	t.Run("refuses to download an export before it is ready", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "csv"}).
			AssertStatus(202).AssertJSONPath("data.url", nil).String("data.id")

		d.Owner.GetJSON(dataExports+"/"+id+"/download").AssertStatus(409).AssertJSONPath("code", "export_not_ready")
	})
}
