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

URL connects as the owning role, without superuser rights. AdminURL
connects to the same database as the superuser, for setup that needs it.
*/
type Database struct {
	URL      string
	AdminURL string
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

	tb.Cleanup(func() {
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

	return Database{
		URL:      withDatabase(tb, serverURL, name, url.UserPassword(name, password)),
		AdminURL: withDatabase(tb, serverURL, name, nil),
	}
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
