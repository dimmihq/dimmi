/*
|--------------------------------------------------------------------------
| Template Databases
|--------------------------------------------------------------------------
|
| Running every migration for every test would make the suite slow, so
| the migrations run once into a template database and each test gets a
| copy of it, which PostgreSQL makes in milliseconds. The template is
| named after a version the caller derives from its migrations, so a
| schema change builds a new template and an unchanged schema reuses the
| one already on the server, even across runs.
|
| Many test processes start at once, so the template is built under an
| advisory lock on the server: the first process builds it, the others
| wait and then copy it. A template is only marked ready once its
| migrations have finished, so a build that died half way is dropped
| and redone.
|
| The migrations run as a role without superuser rights, like the
| application. Each copy is handed to the test's own role, so the test
| owns every table exactly as it would after migrating by itself.
| Extensions PostgreSQL only lets a superuser create, such as pgvector,
| are installed first by the superuser, as an operator does once on a
| real server, so a migration's CREATE EXTENSION IF NOT EXISTS passes.
|
*/

package testkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

/*
Migrate brings the empty database at the URL to the current schema. The
URL connects as the database's owner, without superuser rights. It must
close every connection it opens before it returns.
*/
type Migrate func(ctx context.Context, databaseURL string) error

/*
NewMigratedDatabase returns a database for the test copied from the
template migrated for the version, building the template first when no
process has. The version must change whenever the migrations do;
Fingerprint derives one from the migration files.
*/
func NewMigratedDatabase(tb testing.TB, version string, migrate Migrate) Database {
	tb.Helper()

	template := ensureTemplate(tb, version, migrate)
	db := createDatabase(tb, template)

	u, err := url.Parse(db.AdminURL)
	if err != nil {
		tb.Fatalf("parse database url: %v", err)
	}

	admin, err := pgx.Connect(tb.Context(), db.AdminURL)
	if err != nil {
		tb.Fatalf("connect to %s: %v", u.Path, err)
	}

	defer func() { _ = admin.Close(context.WithoutCancel(tb.Context())) }()

	mustExec(tb, admin, fmt.Sprintf("REASSIGN OWNED BY %s TO %s", template, strings.TrimPrefix(u.Path, "/")))

	return db
}

/*
Fingerprint returns a version for NewMigratedDatabase that changes
whenever a file in any of the file systems is added, removed, renamed
or edited.
*/
func Fingerprint(tb testing.TB, fsyss ...fs.FS) string {
	tb.Helper()

	hash := sha256.New()

	for _, fsys := range fsyss {
		err := fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}

			contents, err := fs.ReadFile(fsys, path)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(hash, "%s\x00%d\x00", path, len(contents))
			_, _ = hash.Write(contents)

			return nil
		})
		if err != nil {
			tb.Fatalf("fingerprint migrations: %v", err)
		}
	}

	return hex.EncodeToString(hash.Sum(nil))
}

/*
ensureTemplate returns the name of the template for the version, built
and ready to copy. The template database and the role that ran its
migrations share the name.
*/
func ensureTemplate(tb testing.TB, version string, migrate Migrate) string {
	tb.Helper()

	sum := sha256.Sum256([]byte(version))
	name := "tpl_" + hex.EncodeToString(sum[:8])
	serverURL := Env(tb, "DATABASE_URL")

	admin, err := pgx.Connect(tb.Context(), serverURL)
	if err != nil {
		tb.Fatalf("connect to postgres: %v", err)
	}

	defer func() { _ = admin.Close(context.WithoutCancel(tb.Context())) }()

	if templateReady(tb, admin, name) {
		return name
	}

	if _, err := admin.Exec(tb.Context(), "SELECT pg_advisory_lock(hashtextextended($1, 0))", name); err != nil {
		tb.Fatalf("lock template %s: %v", name, err)
	}

	defer func() {
		ctx := context.WithoutCancel(tb.Context())
		_, _ = admin.Exec(ctx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", name)
	}()

	if templateReady(tb, admin, name) {
		return name
	}

	buildTemplate(tb, admin, name, migrate)

	return name
}

func templateReady(tb testing.TB, admin *pgx.Conn, name string) bool {
	tb.Helper()

	var ready bool

	err := admin.QueryRow(tb.Context(),
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1 AND datistemplate)", name).Scan(&ready)
	if err != nil {
		tb.Fatalf("look up template %s: %v", name, err)
	}

	return ready
}

/*
buildTemplate migrates a fresh template database as its own role, then
hands the database itself to the superuser and marks it a template that
takes no connections, which is what makes it ready.
*/
func buildTemplate(tb testing.TB, admin *pgx.Conn, name string, migrate Migrate) {
	tb.Helper()

	password := UniqueName("p")

	var exists bool
	if err := admin.QueryRow(tb.Context(), "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", name).
		Scan(&exists); err != nil {
		tb.Fatalf("look up role %s: %v", name, err)
	}

	mustExec(tb, admin, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))

	role := "CREATE ROLE %s LOGIN NOSUPERUSER PASSWORD '%s'"
	if exists {
		role = "ALTER ROLE %s PASSWORD '%s'"
	}

	mustExec(tb, admin, fmt.Sprintf(role, name, password))

	mustExec(tb, admin, fmt.Sprintf("CREATE DATABASE %s OWNER %s", name, name))

	serverURL := Env(tb, "DATABASE_URL")
	installUntrustedExtensions(tb, withDatabase(tb, serverURL, name, nil))
	if err := migrate(tb.Context(), withDatabase(tb, serverURL, name, url.UserPassword(name, password))); err != nil {
		tb.Fatalf("migrate template %s: %v", name, err)
	}

	if _, err := admin.Exec(tb.Context(),
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()",
		name); err != nil {
		tb.Fatalf("close connections to template %s: %v", name, err)
	}

	mustExec(tb, admin, fmt.Sprintf("ALTER DATABASE %s OWNER TO CURRENT_USER", name))
	mustExec(tb, admin, fmt.Sprintf("ALTER DATABASE %s WITH ALLOW_CONNECTIONS false IS_TEMPLATE true", name))
}

/*
untrustedExtensions are the extensions migrations may name that only a
superuser can create.
*/
func untrustedExtensions() []string {
	return []string{"vector"}
}

/*
installUntrustedExtensions creates, as the superuser, each untrusted
extension the server offers in the database.
*/
func installUntrustedExtensions(tb testing.TB, databaseURL string) {
	tb.Helper()

	conn, err := pgx.Connect(tb.Context(), databaseURL)
	if err != nil {
		tb.Fatalf("connect to template as superuser: %v", err)
	}

	defer func() { _ = conn.Close(context.WithoutCancel(tb.Context())) }()

	for _, extension := range untrustedExtensions() {
		var available bool
		if err := conn.QueryRow(tb.Context(),
			"SELECT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = $1)", extension).
			Scan(&available); err != nil {
			tb.Fatalf("look up extension %s: %v", extension, err)
		}

		if available {
			mustExec(tb, conn, "CREATE EXTENSION IF NOT EXISTS "+extension)
		}
	}
}
