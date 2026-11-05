package tags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/tests"
)

func TestTagRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses a membership without conversation permissions", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acts := d.TC.App.Actions.Inbox
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, nil)
		ctx := t.Context()

		_, err := acts.ListTags.Handle(ctx, tags.ListTagsInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.CreateTag.Handle(ctx, tags.CreateTagInput{Membership: none, Name: "x"})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.TagConversation.Handle(ctx, conversations.TagConversationInput{
			Membership: none, Ref: conversations.Ref{ID: ids.New()}, Tags: tags.Refs{Names: []string{"x"}},
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})

	t.Run("recolors a tag, ignoring the case of the same color", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		updated := tests.Record[appevents.TagUpdated](d.TC)
		path := "/api/v1/tags/" + d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Bug", "color": "#abcdef"}).
			AssertCreated().String("data.id")

		d.Owner.PatchJSON(path, testkit.JSON{"color": "#ABCDEF"}).AssertOK()
		d.Owner.PatchJSON(path, testkit.JSON{"color": "#123456"}).AssertOK().AssertJSONPath("data.color", "#123456")
		assert.Len(t, updated.All(), 1)
	})
}

func TestTagFailures(t *testing.T) {
	t.Parallel()

	for table, requests := range map[string][]string{
		"conversation_tag": {"GET counts", "DELETE tag", "POST conversation tag", "DELETE conversation tag"},
		"contact_tag":      {"GET counts"},
		"company_tag":      {"GET counts"},
		"tags":             {"GET tags", "POST tag", "POST conversation tag"},
		"messages":         {"POST conversation tag from message"},
	} {
		t.Run("answers 500 when "+table+" cannot be used", func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.d.TC.BreakTable(table)

			for _, request := range requests {
				assert.Equal(t, 500, f.send(request), request)
			}
		})
	}

	for _, write := range []struct{ table, kind, request string }{
		{"tags", "UPDATE", "PATCH tag"},
		{"tags", "DELETE", "DELETE tag"},
		{"tags", "INSERT", "POST conversation tag"},
		{"conversations", "UPDATE", "POST conversation tag"},
		{"conversation_tag", "INSERT", "POST conversation tag"},
		{"conversation_tag", "DELETE", "DELETE conversation tag"},
	} {
		t.Run("answers 500 when "+write.table+" refuses "+write.kind, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.d.TC.FailWrites(write.table, write.kind)

			assert.Equal(t, 500, f.send(write.request), write.request)
		})
	}
}

/*
fixture is a desk with a tagged conversation.
*/
type fixture struct {
	d       tests.Desk
	conv    string
	message string
	tag     string
}

/*
newFixture starts a conversation and tags it.
*/
func newFixture(t *testing.T) fixture {
	t.Helper()

	d := tests.NewDesk(t)
	res := d.Customer(t, "bob", "Hi")
	conv := res.String("data.id")
	tag := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/tags", testkit.JSON{"tags": []string{"old"}}).
		AssertOK().String("data.tags.0.id")

	return fixture{d: d, conv: conv, message: res.String("data.last_message.id"), tag: tag}
}

/*
send makes one of the named requests and returns its status.
*/
func (f fixture) send(request string) int {
	owner, conv := f.d.Owner, "/api/v1/conversations/"+f.conv+"/tags"

	switch request {
	case "GET tags":
		return owner.GetJSON("/api/v1/tags").Status
	case "GET counts":
		return owner.GetJSON("/api/v1/tags?include=counts").Status
	case "POST tag":
		return owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "new"}).Status
	case "PATCH tag":
		return owner.PatchJSON("/api/v1/tags/"+f.tag, testkit.JSON{"name": "renamed"}).Status
	case "DELETE tag":
		return owner.Delete("/api/v1/tags/" + f.tag).Status
	case "POST conversation tag":
		return owner.PostJSON(conv, testkit.JSON{"tags": []string{"fresh"}}).Status
	case "POST conversation tag from message":
		return owner.PostJSON(conv, testkit.JSON{"tags": []string{"fresh"}, "message_id": f.message}).Status
	default:
		return owner.Delete(conv + "/" + f.tag).Status
	}
}
