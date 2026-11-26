package migrations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with a migration started from a made-up tool.
*/
type desk struct {
	tests.Desk
	client *testkit.Client
	id     string
}

/*
open starts a desk and a migration, through a token with only the
settings ability, the way the CLI reaches the API.
*/
func open(t *testing.T, opts ...tests.Option) desk {
	t.Helper()

	d := desk{Desk: tests.NewDesk(t, opts...)}
	d.client = d.TC.ActingAsToken(d.TC.Workspace.Owner, "settings:write")

	res := d.client.PostJSON("/api/v1/migrations", testkit.JSON{
		"source": "acme-desk", "source_account": "workspace-1", "name": "From Acme",
	}).AssertCreated()
	d.id = res.String("data.id")

	return d
}

/*
path returns the path of the migration's endpoint.
*/
func (d desk) path(rest string) string {
	return "/api/v1/migrations/" + d.id + rest
}

/*
send sends one batch of records of the kind and returns the response.
*/
func (d desk) send(t *testing.T, kind string, records ...testkit.JSON) *testkit.Response {
	t.Helper()

	return d.client.PostJSON(d.path("/records"), testkit.JSON{"kind": kind, "records": records}).AssertOK()
}

/*
upload uploads a file to the migration and returns its file id.
*/
func (d desk) upload(t *testing.T, name, sourceID string, content []byte) string {
	t.Helper()

	res := d.client.Send(fhttp.MethodPost, d.path("/files?filename="+name+"&source_id="+sourceID), content)
	require.Contains(t, []int{fhttp.StatusCreated, fhttp.StatusOK}, res.Status, res.Body)

	return res.String("data.id")
}

/*
at is a moment of the other tool's history: minutes after nine in the
morning of 3 March 2024.
*/
func at(minutes int) string {
	return time.Date(2024, time.March, 3, 9, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute).
		Format(time.RFC3339)
}

/*
rows returns how many rows of the table the workspace has that match the
condition.
*/
func (d desk) rows(t *testing.T, table, where string) int {
	t.Helper()

	var n int

	sql := "SELECT count(*) FROM " + table + " WHERE workspace_id = $1 AND " + where
	err := d.TC.Operator().QueryRow(t.Context(), sql, d.TC.Workspace.Workspace.ID).Scan(&n)
	require.NoError(t, err)

	return n
}

/*
contact is a contact record.
*/
func contact(id, email string) testkit.JSON {
	return testkit.JSON{"source_id": id, "kind": "user", "email": email, "name": "Ada " + id}
}

/*
conversation is the conversation cv-1 of the customer c-1 with the
entries.
*/
func conversation(entries ...testkit.JSON) testkit.JSON {
	return testkit.JSON{
		"source_id": "cv-1", "contact_ref": "c-1", "subject": "Refund", "created_at": at(0), "entries": entries,
	}
}

/*
message is a message entry by the customer c-1.
*/
func message(id string, minute int, body string) testkit.JSON {
	const contactRef = "c-1"

	return testkit.JSON{
		"source_id": id, "type": "message", "created_at": at(minute), "body": body,
		"author": testkit.JSON{"type": "contact", "ref": contactRef},
	}
}

/*
reply is a teammate's reply entry.
*/
func reply(id string, minute int, body, teammateRef string) testkit.JSON {
	return testkit.JSON{
		"source_id": id, "type": "reply", "created_at": at(minute), "body": body,
		"author": testkit.JSON{"type": "teammate", "ref": teammateRef},
	}
}

/*
event is an event entry by the teammate adm-1.
*/
func event(id string, minute int, ev testkit.JSON) testkit.JSON {
	const teammateRef = "adm-1"

	return testkit.JSON{
		"source_id": id, "type": "event", "created_at": at(minute),
		"author": testkit.JSON{"type": "teammate", "ref": teammateRef}, "event": ev,
	}
}
