package access_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its default inbox and its owner signed in.
*/
type desk struct {
	tc    *tests.TestCase
	inbox factories.InboxWithChannel
	owner *testkit.Client
}

/*
open boots the application with a workspace, its default inbox and the
owner signed in.
*/
func open(t *testing.T) desk {
	t.Helper()

	tc := tests.New(t)
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	return desk{tc: tc, inbox: inbox, owner: in(tc, tc.Owner)}
}

/*
in returns a client signed in as the user, working in the test case's
workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)

	return tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspace)
}

/*
teammate is a member of the workspace, signed in.
*/
type teammate struct {
	user   models.User
	member models.WorkspaceMember
	client *testkit.Client
}

/*
public returns the teammate's public member id.
*/
func (m teammate) public() string {
	return models.PublicID(models.PrefixMember, m.member.ID)
}

/*
join adds a new person to the workspace with the role.
*/
func (d desk) join(t *testing.T, role enums.Role) teammate {
	t.Helper()

	user := d.tc.CreateUser()
	member, err := factories.Member(t.Context(), d.tc.App, factories.Membership{
		Workspace: d.tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return teammate{user: user, member: member, client: in(d.tc, user)}
}

/*
team creates a team and returns its public id.
*/
func (d desk) team(t *testing.T, name string) string {
	t.Helper()

	return d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": name}).AssertStatus(201).String("data.id")
}

/*
customer starts a conversation a customer wrote and returns its public
id.
*/
func (d desk) customer(t *testing.T, name string) string {
	t.Helper()

	inbox := models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)

	return d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": inbox, "author": "contact", "subject": "About " + name, "body": "Words from " + name,
		"contact": testkit.JSON{"email": name + "@example.com", "name": name},
	}).AssertCreated().String("data.id")
}

/*
restrict changes what the member sees, through the API.
*/
func (d desk) restrict(t *testing.T, m teammate, body testkit.JSON) {
	t.Helper()

	d.owner.PatchJSON("/api/v1/members/"+m.public(), body).AssertOK()
}

/*
assign gives the conversation to the team, the member or both.
*/
func (d desk) assign(t *testing.T, conversation string, body testkit.JSON) {
	t.Helper()

	d.owner.PatchJSON("/api/v1/conversations/"+conversation, body).AssertOK()
}

/*
listed returns the ids of the conversations a list response holds.
*/
func listed(t *testing.T, res *testkit.Response) []string {
	t.Helper()

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	res.AssertOK().Decode(&body)

	out := make([]string, 0, len(body.Data))
	for _, row := range body.Data {
		out = append(out, row.ID)
	}

	return out
}
