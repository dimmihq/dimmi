/*
|--------------------------------------------------------------------------
| Desk
|--------------------------------------------------------------------------
|
| A support desk ready for a record test: a workspace with its default
| inbox and API channel, and its owner signed in the way a browser is.
| Customers write in through the real API, so every test starts from the
| data a real team would have.
|
*/

package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
Desk is a test case's workspace with an inbox and its owner signed in.
*/
type Desk struct {
	TC    *TestCase
	Inbox factories.InboxWithChannel
	Owner *testkit.Client
}

/*
NewDesk boots the application with a workspace, its default inbox with
its API channel and the owner signed in.
*/
func NewDesk(t *testing.T) Desk {
	t.Helper()

	tc := New(t)
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	return Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
}

/*
In returns a client signed in as the user, working in the test case's
workspace.
*/
func (tc *TestCase) In(user models.User) *testkit.Client {
	tc.T.Helper()

	return tc.ActingAs(user).
		WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))
}

/*
InboxID returns the public id of the desk's inbox.
*/
func (d Desk) InboxID() string {
	return models.PublicID(models.PrefixInbox, d.Inbox.Inbox.ID)
}

/*
Join adds a new user to the desk's workspace with the role, and returns
a client signed in as them and their membership.
*/
func (d Desk) Join(t *testing.T, role enums.Role) (*testkit.Client, models.WorkspaceMember) {
	t.Helper()

	user := d.TC.CreateUser()
	member, err := factories.Member(t.Context(), d.TC.App, factories.Membership{
		Workspace: d.TC.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return d.TC.In(user), member
}

/*
Customer starts a conversation the customer named wrote, through the API
channel, and returns the response.
*/
func (d Desk) Customer(t *testing.T, name, subject string) *testkit.Response {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": subject, "body": "Hello from " + name,
		"contact": testkit.JSON{"email": name + "@example.com", "name": name},
	}).AssertCreated()
}

/*
Recorder keeps every event of one type the application dispatched after
its transaction committed.
*/
type Recorder[E any] struct {
	mu   sync.Mutex
	seen []E
}

/*
Record starts recording the events of type E.
*/
func Record[E any](tc *TestCase) *Recorder[E] {
	r := &Recorder[E]{}
	events.Listen(tc.App.Events, events.AfterCommit, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.seen = append(r.seen, e)

		return nil
	})

	return r
}

/*
All returns the recorded events.
*/
func (r *Recorder[E]) All() []E {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]E{}, r.seen...)
}
