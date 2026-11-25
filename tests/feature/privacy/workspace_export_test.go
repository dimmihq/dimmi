package privacy_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/dataexports"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
archiveManifest is manifest.json as the tests read it.
*/
type archiveManifest struct {
	FormatVersion string `json:"format_version"`
	Workspace     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	GeneratedAt string `json:"generated_at"`
	Files       []struct {
		Name        string `json:"name"`
		Records     int    `json:"records"`
		Description string `json:"description"`
	} `json:"files"`
	Attachments  int `json:"attachments"`
	SkippedFiles []struct {
		Reason string `json:"reason"`
	} `json:"skipped_files"`
}

/*
builtExport is a ready export as the tests read it.
*/
type builtExport struct {
	Data struct {
		RowCount  int      `json:"row_count"`
		SizeBytes int      `json:"size_bytes"`
		Warnings  []string `json:"warnings"`
	} `json:"data"`
}

/*
workspaceArchive asks for an export of the whole workspace, builds it
and returns the ready export with the files of its zip.
*/
func workspaceArchive(t *testing.T, d tests.Desk) (builtExport, map[string]string) {
	t.Helper()

	id := d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "workspace", "format": "zip"}).AssertStatus(202).
		AssertJSONPath("data.kind", "workspace").AssertJSONPath("data.include_notes", true).String("data.id")
	d.TC.Drain()

	export := d.Owner.GetJSON(dataExports+"/"+id).AssertOK().AssertJSONPath("data.status", "ready").
		AssertJSONPath("data.file_name", "workspace-export.zip")

	raw, err := d.TC.App.IDs.Decode(string(models.PrefixDataExport), id)
	require.NoError(t, err)

	disk, err := d.TC.App.Storage.Disk(d.TC.App.Application.Config.Filesystems.Default)
	require.NoError(t, err)

	path := "workspaces/" + d.TC.Workspace.Workspace.ID.String() + "/data-exports/" + raw.String() +
		"/workspace-export.zip"
	body, err := disk.Get(t.Context(), path)
	require.NoError(t, err)

	var built builtExport
	export.Decode(&built)
	assert.Equal(t, len(body), built.Data.SizeBytes)

	return built, unzip(t, body)
}

/*
lines returns the JSON lines of one file of the archive.
*/
func lines(t *testing.T, files map[string]string, name string) []string {
	t.Helper()

	contents, ok := files[name]
	require.True(t, ok, "%s is missing from the archive", name)

	if contents == "" {
		return nil
	}

	out := strings.Split(strings.TrimSuffix(contents, "\n"), "\n")
	for _, line := range out {
		require.True(t, json.Valid([]byte(line)), "%s holds a line that is not JSON: %s", name, line)
	}

	return out
}

/*
seedRow runs a statement that seeds or inspects rows as an operator.
*/
func seedRow(t *testing.T, tc *tests.TestCase, sql string, args ...any) {
	t.Helper()

	_, err := tc.Operator().Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func TestWorkspaceExport(t *testing.T) {
	t.Parallel()

	t.Run("writes a manifest, a file of JSON lines per table and the stored files", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace
		path := "/api/v1/conversations/" + d.Customer(t, "ann", "Order").String("data.id")
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "VIP, be gentle"}).AssertCreated()
		storeAttachment(t, d.TC, path)
		seedRow(t, d.TC, `UPDATE attachments SET scan_status = 'clean' WHERE workspace_id = $1`, ws.ID)
		seedRow(t, d.TC, `
			INSERT INTO attachments
				(workspace_id, uploaded_by_type, disk, path, filename, content_type, size_bytes, scan_status)
			VALUES ($1, 'contact', $2, $3, 'virus.exe', 'application/octet-stream', 5, 'infected'),
			       ($1, 'contact', $2, $4, 'lost.pdf', 'application/pdf', 5, 'clean')`,
			ws.ID, d.TC.App.Application.Config.Filesystems.Default,
			"workspaces/"+ws.ID.String()+"/infected.exe", "workspaces/"+ws.ID.String()+"/lost.pdf")

		export, files := workspaceArchive(t, d)

		var manifest archiveManifest
		require.NoError(t, json.Unmarshal([]byte(files["manifest.json"]), &manifest))

		assert.Equal(t, "1", manifest.FormatVersion)
		assert.Equal(t, ws.ID.String(), manifest.Workspace.ID)
		assert.Equal(t, ws.Name, manifest.Workspace.Name)
		assert.NotEmpty(t, manifest.GeneratedAt)
		assert.Len(t, manifest.Files, len(dataexports.ArchiveTables()))

		total := 0

		for _, entry := range manifest.Files {
			assert.NotEmpty(t, entry.Description, entry.Name)
			assert.Len(t, lines(t, files, entry.Name), entry.Records, entry.Name)
			total += entry.Records
		}

		assert.Equal(t, total, export.Data.RowCount)
		assert.Len(t, lines(t, files, "workspaces.ndjson"), 1)
		assert.Len(t, lines(t, files, "conversations.ndjson"), 1)
		assert.Contains(t, files["messages.ndjson"], "Hello from ann")
		assert.Contains(t, files["messages.ndjson"], "VIP, be gentle")
		assert.Contains(t, files["contacts.ndjson"], "ann@example.com")
		assert.Contains(t, files["workspace_members.ndjson"], d.TC.Owner.Email)
		assert.Contains(t, files["workspace_members.ndjson"], `"role": "owner"`)

		kept := map[string]string{}
		for _, line := range lines(t, files, "attachments.ndjson") {
			var row struct {
				Filename    string  `json:"filename"`
				ArchivePath string  `json:"archive_path"`
				Path        *string `json:"path"`
			}
			require.NoError(t, json.Unmarshal([]byte(line), &row))

			kept[row.Filename] = row.ArchivePath
			assert.Nil(t, row.Path)
		}

		require.Len(t, kept, 3)
		assert.True(t, strings.HasPrefix(kept["receipt.txt"], "attachments/attachments/"))
		assert.True(t, strings.HasSuffix(kept["receipt.txt"], ".txt"))
		assert.Equal(t, "receipt", files[kept["receipt.txt"]])
		assert.Empty(t, kept["virus.exe"])
		assert.Empty(t, kept["lost.pdf"])
		assert.Equal(t, 1, manifest.Attachments)

		reasons := map[string]bool{}
		for _, skipped := range manifest.SkippedFiles {
			reasons[skipped.Reason] = true
		}

		assert.Equal(t, map[string]bool{"scan infected": true, "missing from storage": true}, reasons)
		assert.Len(t, export.Data.Warnings, 1)
	})

	t.Run("never holds another workspace's rows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "Order")
		other, err := factories.Workspace(t.Context(), d.TC.App, d.TC.Owner)
		require.NoError(t, err)

		for _, ws := range []struct {
			id   any
			name string
		}{{d.TC.Workspace.Workspace.ID, "ours-kept"}, {other.Workspace.ID, "theirs-foreign"}} {
			seedRow(t, d.TC, `INSERT INTO tags (workspace_id, name) VALUES ($1, $2)`, ws.id, ws.name)
			seedRow(t, d.TC, `INSERT INTO companies (workspace_id, name) VALUES ($1, $2)`, ws.id, ws.name)
		}

		_, files := workspaceArchive(t, d)

		assert.Contains(t, files["tags.ndjson"], "ours-kept")
		assert.Contains(t, files["companies.ndjson"], "ours-kept")
		assert.Len(t, lines(t, files, "workspaces.ndjson"), 1)

		for name, contents := range files {
			assert.NotContains(t, contents, "theirs-foreign", name)
			assert.NotContains(t, contents, other.Workspace.ID.String(), name)
		}
	})
}

func TestWorkspaceExportBounds(t *testing.T) {
	t.Parallel()

	t.Run("writes every row of tables longer than a page, once each", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		d.Customer(t, "ann", "Order")
		seedRow(t, d.TC, `
			INSERT INTO tags (workspace_id, name) SELECT $1, 'bulk-' || n FROM generate_series(1, 2500) AS n`, ws)
		seedRow(t, d.TC, `
			INSERT INTO conversation_tag (workspace_id, conversation_id, tag_id, created_by_type)
			SELECT t.workspace_id, c.id, t.id, 'system' FROM tags t JOIN conversations c USING (workspace_id)
			WHERE t.workspace_id = $1`, ws)

		_, files := workspaceArchive(t, d)

		for _, name := range []string{"tags.ndjson", "conversation_tag.ndjson"} {
			seen := map[string]bool{}
			for _, line := range lines(t, files, name) {
				seen[line] = true
			}

			assert.Len(t, seen, 2500, name)
			assert.Len(t, lines(t, files, name), 2500, name)
		}
	})

	t.Run("never holds a secret, sealed or plain", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		secret := d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": "https://hooks.example.com/dimmi", "event_types": []string{"conversation.created"},
		}).AssertCreated().String("meta.secret")
		require.NotEmpty(t, secret)

		sealed := map[string][]byte{
			"ai key": []byte("sealed-ai-provider-key-bytes"), "integration": []byte("sealed-integration-token-bytes"),
			"channel": []byte("sealed-channel-credential-bytes"),
		}
		seedRow(t, d.TC, `
			INSERT INTO ai_provider_keys (workspace_id, name, provider, credentials_encrypted, key_hint)
			VALUES ($1, 'Anthropic', 'anthropic', $2, 'hint-7f3q')`, ws, sealed["ai key"])
		seedRow(t, d.TC, `
			INSERT INTO integration_installations (workspace_id, provider, name, source, credentials_encrypted)
			VALUES ($1, 'slack', 'Acme Slack', 'oauth', $2)`, ws, sealed["integration"])
		seedRow(t, d.TC, `UPDATE channel_connections SET credentials_encrypted = $2 WHERE workspace_id = $1`,
			ws, sealed["channel"])

		var hook []byte
		require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
			`SELECT secret_encrypted FROM webhook_endpoints WHERE workspace_id = $1`, ws).Scan(&hook))
		require.NotEmpty(t, hook)

		sealed["webhook"] = hook

		_, files := workspaceArchive(t, d)

		assert.Contains(t, files["webhook_endpoints.ndjson"], "https://hooks.example.com/dimmi")
		assert.NotEmpty(t, lines(t, files, "channel_connections.ndjson"))

		for name, contents := range files {
			assert.NotContains(t, contents, secret, name)
			assert.NotContains(t, contents, "hint-7f3q", name)
			assert.NotContains(t, contents, "_encrypted", name)

			for what, value := range sealed {
				assert.NotContains(t, contents, string(value), "%s in %s", what, name)
				assert.NotContains(t, contents, hex.EncodeToString(value), "%s in %s", what, name)
				assert.NotContains(t, contents, base64.StdEncoding.EncodeToString(value), "%s in %s", what, name)
			}
		}
	})

	t.Run("goes only to owners, as a zip, and links to its file", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		admin, _ := d.Join(t, enums.RoleAdmin)
		body := testkit.JSON{"kind": "workspace", "format": "zip"}

		admin.PostJSON(dataExports, body).AssertForbidden()
		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "workspace", "format": "csv"}).AssertStatus(422).
			AssertJSONPath("code", "invalid_export_format")
		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "contacts", "format": "zip"}).AssertStatus(422).
			AssertJSONPath("code", "invalid_export_format")

		d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read").PostJSON(dataExports, body).AssertForbidden()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read", "exports:write").
			PostJSON(dataExports, body).AssertStatus(202)

		id := d.Owner.PostJSON(dataExports, body).AssertStatus(202).String("data.id")
		d.Owner.GetJSON(dataExports+"/"+id+"/link").AssertStatus(409).AssertJSONPath("code", "export_not_ready")
		admin.GetJSON(dataExports + "/" + id + "/link").AssertNotFound()
		d.TC.Drain()

		link := d.Owner.GetJSON(dataExports+"/"+id+"/link").AssertOK().
			AssertJSONPath("data.object", "data_export_link").String("data.url")
		redirect := d.Owner.GetJSON(dataExports + "/" + id + "/download").AssertStatus(307)
		assert.NotEmpty(t, link)
		assert.NotEmpty(t, redirect.Header.Get("Location"))
		assert.Empty(t, redirect.Body)

		d.TC.Table("audit_logs").AssertHas(map[string]any{"event": "data_export.requested"})
		d.TC.Table("audit_logs").AssertHas(map[string]any{"event": "data_export.ready"})
	})
}

func TestWorkspaceExportRegistry(t *testing.T) {
	t.Parallel()

	t.Run("exports or leaves out, on purpose, every table a workspace owns", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exported := map[string]bool{}

		for _, table := range dataexports.ArchiveTables() {
			assert.False(t, exported[table.Name], "%s is listed twice", table.Name)
			exported[table.Name] = true
		}

		excluded := dataexports.ArchiveExclusions()
		owned := map[string]bool{"workspaces": true}

		for _, column := range schemaColumns(t, tc, `c.column_name = 'workspace_id'`) {
			owned[column.table] = true
			reason, out := excluded[column.table]

			assert.True(t, exported[column.table] != out, "%s has a workspace_id: export it in "+
				"dataexports.ArchiveTables or leave it out with a reason in dataexports.ArchiveExclusions, "+
				"and never both", column.table)
			assert.True(t, !out || reason != "", "%s is left out without a reason", column.table)
		}

		for table := range exported {
			assert.True(t, owned[table], "%s is exported but is not a table a workspace owns", table)
		}

		for table := range excluded {
			assert.True(t, owned[table], "%s is left out but is not a table a workspace owns", table)
		}
	})
}

func TestWorkspaceExportSecretColumns(t *testing.T) {
	t.Parallel()

	t.Run("exports no column that holds a secret", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		spared := map[string]bool{
			"ai_agents.max_output_tokens": true, "webhook_endpoints.previous_secret_expires_at": true,
		}
		secretive := []string{"secret", "token", "password", "credential", "encrypted", "private_key", "nonce"}
		exported := map[string]bool{}

		for _, table := range dataexports.ArchiveTables() {
			exported[table.Name] = true

			for _, column := range table.Omit {
				spared[table.Name+"."+column] = true
			}
		}

		for _, column := range schemaColumns(t, tc, `c.udt_name NOT IN ('tsvector', 'halfvec')`) {
			name := column.table + "." + column.name
			if !exported[column.table] || spared[name] {
				continue
			}

			assert.NotEqual(t, "bytea", column.kind, "%s holds bytes: name it in the table's Omit", name)

			for _, word := range secretive {
				assert.NotContains(t, column.name, word, "%s reads like a secret: name it in the table's Omit", name)
			}
		}
	})
}

/*
schemaColumn is one column of a table of the schema.
*/
type schemaColumn struct {
	table string
	name  string
	kind  string
}

/*
schemaColumns returns the columns of the schema's tables the condition
keeps.
*/
func schemaColumns(t *testing.T, tc *tests.TestCase, condition string) []schemaColumn {
	t.Helper()

	rows, err := tc.Operator().Query(t.Context(), `
		SELECT c.table_name::text, c.column_name::text, c.udt_name::text FROM information_schema.columns c
		JOIN information_schema.tables t USING (table_schema, table_name)
		WHERE c.table_schema = current_schema() AND t.table_type = 'BASE TABLE' AND `+condition+`
		ORDER BY 1, 2`)
	require.NoError(t, err)

	defer rows.Close()

	var out []schemaColumn

	for rows.Next() {
		var column schemaColumn
		require.NoError(t, rows.Scan(&column.table, &column.name, &column.kind))

		out = append(out, column)
	}

	require.NoError(t, rows.Err())

	return out
}
