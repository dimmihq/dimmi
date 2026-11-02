package migrate_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database/migrate"
)

func rules(findings []migrate.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Path+":"+f.Rule)
	}

	return out
}

func file(header, up string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(header + "\n-- +goose Up\n" + up + "\n-- +goose Down\nDROP TABLE anything;\n")}
}

const (
	expand   = "-- migrate: phase=expand"
	contract = "-- migrate: phase=contract"
)

func TestLint(t *testing.T) {
	t.Parallel()

	t.Run("accepts safe expand and contract migrations", func(t *testing.T) {
		t.Parallel()

		findings := migrate.Lint(fstest.MapFS{
			"001_create.sql": file(expand, `CREATE TABLE users (id int, email text);
				CREATE UNIQUE INDEX users_email ON users (email);
				ALTER TABLE users ADD COLUMN created_at timestamptz DEFAULT now();`),
			"002_index.sql": file(expand, `DROP INDEX CONCURRENTLY IF EXISTS users_id;
				CREATE INDEX CONCURRENTLY IF NOT EXISTS users_id ON users (id);
				ALTER TABLE users ADD CONSTRAINT email_present CHECK (email IS NOT NULL) NOT VALID;`),
			"003_drop.sql": file(contract,
				"ALTER TABLE users DROP COLUMN legacy; ALTER TABLE users RENAME TO people;"),
			"004_not_null.sql": file(contract, "ALTER TABLE people ALTER COLUMN email SET NOT NULL;"),
			"notes.txt":        {Data: []byte("not a migration")},
			"nested":           {Mode: os.ModeDir},
		})

		assert.Empty(t, findings)
	})

	t.Run("flags each risky statement", func(t *testing.T) {
		t.Parallel()

		findings := migrate.Lint(fstest.MapFS{
			"001_header.sql":   file("-- migrate: phase=later", "SELECT 1;"),
			"002_drop.sql":     file(expand, "-- a comment saying DROP is fine\nALTER TABLE users DROP COLUMN legacy;"),
			"003_rename.sql":   file(expand, "ALTER TABLE users RENAME COLUMN a TO b;"),
			"004_type.sql":     file(contract, "ALTER TABLE users ALTER COLUMN id SET DATA TYPE bigint;"),
			"005_not_null.sql": file(contract, "ALTER TABLE users ALTER email SET NOT NULL;"),
			"006_default.sql":  file(expand, "ALTER TABLE users ADD COLUMN token uuid DEFAULT gen_random_uuid();"),
			"007_serial.sql":   file(expand, "ALTER TABLE users ADD COLUMN seq bigserial;"),
			"008_index.sql":    file(expand, "/* busy table */ CREATE INDEX users_email ON ONLY public.users (email);"),
		})

		assert.Equal(t, []string{
			"001_header.sql:header",
			"002_drop.sql:contract-in-expand",
			"003_rename.sql:contract-in-expand",
			"004_type.sql:column-type",
			"005_not_null.sql:set-not-null",
			"006_default.sql:volatile-default",
			"007_serial.sql:volatile-default",
			"008_index.sql:blocking-index",
		}, rules(findings))
		assert.Contains(t, findings[4].Message, "CHECK (email IS NOT NULL) NOT VALID")
		assert.Contains(t, findings[7].Message, "public.users")
	})

	t.Run("reports files it cannot read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "001_secret.sql")
		require.NoError(t, os.WriteFile(path, file(expand, "SELECT 1;").Data, 0o600))
		require.NoError(t, os.Chmod(path, 0))

		assert.Equal(t, []string{"001_secret.sql:read"}, rules(migrate.Lint(os.DirFS(dir))))
		assert.Equal(t, []string{".:read"}, rules(migrate.Lint(os.DirFS(filepath.Join(dir, "missing")))))
	})
}
