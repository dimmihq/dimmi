package conversations_test

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
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a test case's workspace with an inbox and its owner signed in.
*/
type desk struct {
	tc    *tests.TestCase
	inbox factories.InboxWithChannel
	owner *testkit.Client
}

/*
open boots the application with a workspace, its default inbox with
its API channel and the owner signed in.
*/
func open(t *testing.T, opts ...tests.Option) desk {
	t.Helper()

	tc := tests.New(t, opts...)
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	return desk{tc: tc, inbox: inbox, owner: in(tc, tc.Owner)}
}

/*
in returns a client signed in as the user, working in the test case's
workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	return tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspaceID(tc))
}

/*
workspaceID returns the public id of the test case's workspace.
*/
func workspaceID(tc *tests.TestCase) string {
	return models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
}

/*
inboxID returns the public id of the desk's inbox.
*/
func (d desk) inboxID() string {
	return models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)
}

/*
ownerID returns the public id of the owner's membership.
*/
func (d desk) ownerID() string {
	return models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID)
}

/*
join adds a new user to the desk's workspace with the role, and returns
a client signed in as them and their membership.
*/
func (d desk) join(t *testing.T, role enums.Role) (*testkit.Client, models.WorkspaceMember) {
	t.Helper()

	user := d.tc.CreateUser()
	member, err := factories.Member(t.Context(), d.tc.App, factories.Membership{
		Workspace: d.tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return in(d.tc, user), member
}

/*
customer starts a conversation the customer wrote, through the API
channel, and returns its public id.
*/
func (d desk) customer(t *testing.T, name, subject, body string) string {
	t.Helper()

	res := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.inboxID(), "author": "contact", "subject": subject, "body": body,
		"contact": testkit.JSON{"email": name + "@example.com", "name": name},
	}).AssertCreated()

	return res.String("data.id")
}

/*
memberID returns the public id of the member.
*/
func memberID(m models.WorkspaceMember) string {
	return models.PublicID(models.PrefixMember, m.ID)
}

/*
recorder keeps every event of one type the application dispatched after
its transaction committed.
*/
type recorder[E any] struct {
	mu   sync.Mutex
	seen []E
}

/*
record starts recording the events of type E.
*/
func record[E any](tc *tests.TestCase) *recorder[E] {
	r := &recorder[E]{}
	events.Listen(tc.App.Events, events.AfterCommit, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.seen = append(r.seen, e)

		return nil
	})

	return r
}

/*
all returns the recorded events.
*/
func (r *recorder[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]E{}, r.seen...)
}
