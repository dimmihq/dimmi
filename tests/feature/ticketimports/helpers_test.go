package ticketimports_test

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ticketimports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
	intercomfixtures "github.com/dimmihq/dimmi/tests/fixtures/intercom-tickets"
	zendeskfixtures "github.com/dimmihq/dimmi/tests/fixtures/zendesk-tickets"
)

/*
importCase is a desk with Intercom and Zendesk faked from their
recorded answers.
*/
type importCase struct {
	tests.Desk
	created *tests.Recorder[appevents.TicketCreated]

	mu    sync.Mutex
	calls []string
}

/*
newImportCase starts a desk and fakes both tools.
*/
func newImportCase(t *testing.T) *importCase {
	t.Helper()

	c := &importCase{Desk: tests.NewDesk(t)}
	c.created = tests.Record[appevents.TicketCreated](c.TC)
	owner := c.TC.Owner.Email

	httpclienttest.New(c.TC.App.HTTP).
		Serve("https://api.intercom.io/*", c.serve(intercomfixtures.Files, owner, intercomFile)).
		Serve("https://acme.zendesk.com/*", c.serve(zendeskfixtures.Files, owner, zendeskFile))

	return c
}

/*
serve answers from the fixtures, the owner's email in place of
{{owner}}.
*/
func (c *importCase) serve(files fs.FS, owner string, name func(*fhttp.Request, string) string) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
		}

		c.mu.Lock()
		c.calls = append(c.calls, r.Method+" "+r.URL.Path)
		c.mu.Unlock()

		raw, err := fs.ReadFile(files, name(r, string(body)))
		if err != nil {
			w.WriteHeader(fhttp.StatusNotFound)
			_, _ = io.WriteString(w, `{"errors": [{"code": "not_found"}]}`)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.ReplaceAll(string(raw), "{{owner}}", owner))
	})
}

/*
intercomFile names the fixture of an Intercom call.
*/
func intercomFile(r *fhttp.Request, body string) string {
	path := strings.Trim(r.URL.Path, "/")
	if path == "tickets/search" {
		if strings.Contains(body, "starting_after") {
			return "tickets_search.page2.json"
		}

		return "tickets_search.page1.json"
	}

	return strings.ReplaceAll(path, "/", "_") + ".json"
}

/*
zendeskFile names the fixture of a Zendesk call.
*/
func zendeskFile(r *fhttp.Request, _ string) string {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/")
	if path == "incremental/tickets/cursor.json" {
		if r.URL.Query().Get("cursor") != "" {
			return "export.page2.json"
		}

		return "export.page1.json"
	}

	if strings.HasPrefix(path, "tickets/") {
		return "audits_" + strings.Split(path, "/")[1] + ".json"
	}

	return path
}

/*
owner returns the owner's membership with the permissions to import.
*/
func (c *importCase) owner() authz.Membership {
	return authz.NewMembership(c.TC.Workspace.Workspace, c.TC.Workspace.Owner,
		[]string{"tickets:read", "tickets:write", "tickets:manage"})
}

/*
intercom returns an Intercom source.
*/
func (c *importCase) intercom() *ticketimports.Intercom {
	return &ticketimports.Intercom{HTTP: c.TC.App.HTTP, Token: "intercom-test-token"}
}

/*
zendesk returns a Zendesk source.
*/
func (c *importCase) zendesk() *ticketimports.Zendesk {
	return &ticketimports.Zendesk{
		HTTP: c.TC.App.HTTP, BaseURL: "https://acme.zendesk.com", Email: "admin@acme.test", Token: "zd-test",
	}
}

/*
run imports from the source and requires it to complete.
*/
func (c *importCase) run(t *testing.T, src ticketimports.Source) models.TicketImport {
	t.Helper()

	row, err := c.TC.App.Actions.TicketImports.Import(t.Context(), c.owner(), src)
	require.NoError(t, err)
	require.Equal(t, "completed", row.Status, "%v", row.Error)

	return row
}

/*
imported is a ticket brought in, read back with its type, state and
history.
*/
type imported struct {
	Ticket models.Ticket
	Type   models.TicketType
	State  models.TicketState
	Events []models.TicketEvent
}

/*
ticket reads the ticket with the external id.
*/
func (c *importCase) ticket(t *testing.T, external string) imported {
	t.Helper()

	ws := c.TC.Workspace.Workspace.ID

	var out imported

	ctx := tenant(t, ws)
	q := models.New(c.TC.Operator())

	var err error

	out.Ticket, err = q.FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{
		WorkspaceID: ws, ExternalID: &external,
	})
	require.NoError(t, err, external)

	out.Type, err = q.FindTicketType(ctx, models.FindTicketTypeParams{WorkspaceID: ws, ID: out.Ticket.TypeID})
	require.NoError(t, err)

	out.State, err = q.FindTicketState(ctx, models.FindTicketStateParams{WorkspaceID: ws, ID: out.Ticket.StateID})
	require.NoError(t, err)

	out.Events, err = q.ListTicketEvents(ctx, models.ListTicketEventsParams{
		WorkspaceID: ws, TicketID: out.Ticket.ID, RowLimit: 100,
	})
	require.NoError(t, err)

	return out
}

/*
count runs a count query against the workspace.
*/
func (c *importCase) count(t *testing.T, sql string, args ...any) int {
	t.Helper()

	var n int

	all := append([]any{c.TC.Workspace.Workspace.ID}, args...)
	require.NoError(t, c.TC.Operator().QueryRow(t.Context(), sql, all...).Scan(&n))

	return n
}

/*
state returns the workspace's live state with the name.
*/
func (c *importCase) state(t *testing.T, name string) models.TicketState {
	t.Helper()

	rows, err := models.New(c.TC.Operator()).ListTicketStates(tenant(t, c.TC.Workspace.Workspace.ID),
		c.TC.Workspace.Workspace.ID)
	require.NoError(t, err)

	for _, s := range rows {
		if s.Name == name && s.ArchivedAt == nil {
			return s
		}
	}

	t.Fatalf("no state %s", name)

	return models.TicketState{}
}

/*
tenant returns the test's context, unchanged: the operator role reads
every workspace.
*/
func tenant(t *testing.T, _ ids.UUID) context.Context {
	t.Helper()

	return t.Context()
}

/*
at reads a time written as RFC 3339.
*/
func at(s string) time.Time {
	v, _ := time.Parse(time.RFC3339, s)

	return v
}
