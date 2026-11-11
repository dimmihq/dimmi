/*
|--------------------------------------------------------------------------
| Test Databases
|--------------------------------------------------------------------------
|
| Each test gets a database of its own, owned by a fresh role without
| superuser rights. Tests can run in parallel without seeing each other's
| rows, and row-level security applies to them exactly as it does in
| production, where the application never connects as a superuser.
|
| Every database and role a test creates is dropped when the test ends,
| whether it passed, failed or stopped half way, so test runs leave
| nothing behind on the server.
|
| DATABASE_URL must connect as a superuser, since it creates the roles.
|
*/

package testkit

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

/*
Database is a throwaway database created for one test.

Name is the database's name, which its owning role shares. URL connects
as the owning role, without superuser rights. AdminURL connects to the
same database as the superuser, for setup that needs it.
*/
type Database struct {
	Name     string
	URL      string
	AdminURL string
}

/*
Role is a login role created for one test. URL connects as it to the
database it was created for.
*/
type Role struct {
	Name string
	URL  string
}

/*
NewDatabase creates a database and its owner for the test, and drops
both when the test ends.
*/
func NewDatabase(tb testing.TB) Database {
	tb.Helper()

	return createDatabase(tb, "")
}

/*
createDatabase creates a database and its owner for the test, copied
from the template when one is named, and drops both when the test ends.
An empty database gets the extensions only a superuser may create, as
a template does, so migrations run on it the same way.
*/
func createDatabase(tb testing.TB, template string) Database {
	tb.Helper()

	serverURL := Env(tb, "DATABASE_URL")
	name := UniqueName("t_")
	password := UniqueName("p")

	admin, err := pgx.Connect(tb.Context(), serverURL)
	if err != nil {
		tb.Fatalf("connect to postgres: %v", err)
	}

	Cleanup(tb, func() {
		ctx := context.WithoutCancel(tb.Context())
		_, _ = admin.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS "+name)
		_ = admin.Close(ctx)
	})

	mustExec(tb, admin, fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER PASSWORD '%s'", name, password))

	create := fmt.Sprintf("CREATE DATABASE %s OWNER %s", name, name)
	if template != "" {
		create += " TEMPLATE " + template
	}

	mustExec(tb, admin, create)

	if template == "" {
		installUntrustedExtensions(tb, withDatabase(tb, serverURL, name, nil))
	}

	return Database{
		Name:     name,
		URL:      withDatabase(tb, serverURL, name, url.UserPassword(name, password)),
		AdminURL: withDatabase(tb, serverURL, name, nil),
	}
}

/*
NewRole creates a login role without superuser rights that owns nothing
in the database, the way an application role that is not the tables'
owner connects, so row level security applies to it. Each grant names
privileges the role gets, such as "SELECT ON ALL TABLES IN SCHEMA
public". When the test ends, whatever its outcome, the role's privileges
in the database and the role itself are dropped, before the database
is.
*/
func (d Database) NewRole(tb testing.TB, grants ...string) Role {
	tb.Helper()

	name := UniqueName("t_")
	password := UniqueName("p")

	admin, err := pgx.Connect(tb.Context(), d.AdminURL)
	if err != nil {
		tb.Fatalf("connect to %s: %v", d.Name, err)
	}

	Cleanup(tb, func() {
		ctx := context.WithoutCancel(tb.Context())
		_, _ = admin.Exec(ctx, "DROP OWNED BY "+name)
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS "+name)
		_ = admin.Close(ctx)
	})

	mustExec(tb, admin, fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER PASSWORD '%s'", name, password))

	for _, grant := range grants {
		mustExec(tb, admin, "GRANT "+grant+" TO "+name)
	}

	return Role{Name: name, URL: withDatabase(tb, d.AdminURL, d.Name, url.UserPassword(name, password))}
}

func mustExec(tb testing.TB, conn *pgx.Conn, sql string) {
	tb.Helper()

	if _, err := conn.Exec(tb.Context(), sql); err != nil {
		tb.Fatalf("%s: %v", sql, err)
	}
}

func withDatabase(tb testing.TB, serverURL, database string, user *url.Userinfo) string {
	tb.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		tb.Fatalf("parse DATABASE_URL: %v", err)
	}

	u.Path = "/" + database
	if user != nil {
		u.User = user
	}

	return u.String()
}
