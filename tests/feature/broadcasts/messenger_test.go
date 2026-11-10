package broadcasts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
messengerVisitor starts a Messenger session for a signed-up person and
returns their bearer token.
*/
func messengerVisitor(d desk, email string) string {
	d.tc.T.Helper()

	var widgetID string

	ws := d.tc.Workspace.Workspace.ID
	query(d, func(ctx context.Context, q *models.Queries) {
		rows, err := q.ListWidgetSettings(ctx, ws)
		require.NoError(d.tc.T, err)
		require.NotEmpty(d.tc.T, rows)
		widgetID = models.PublicID(models.PrefixWidget, rows[0].ID)
	})

	res := d.tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": widgetID}).AssertCreated()
	token := res.String("data.token")
	d.tc.Client().WithHeader("Authorization", "Bearer "+token).
		PatchJSON("/widget/v1/contact", testkit.JSON{"email": email, "name": "Ada Okafor"}).AssertOK()

	return token
}

func TestMessengerBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("a chat reaches people in the Messenger and their answer becomes a conversation", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ada := unique("ada")
		token := messengerVisitor(d, ada)
		visitor := d.tc.Client().WithHeader("Authorization", "Bearer "+token)

		id := d.owner.PostJSON("/api/v1/broadcasts", testkit.JSON{"name": "Onboarding call", "channel": "chat"}).
			AssertCreated().String("data.id")
		d.owner.PatchJSON("/api/v1/broadcasts/"+id, testkit.JSON{
			"body":            "Hi {{ contact.first_name }}, want a hand getting set up?",
			"rules":           testkit.JSON{"rules": testkit.JSON{"field": "contact.email", "op": "eq", "value": ada}},
			"sender_id":       models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID),
			"reply_member_id": models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID),
		}).AssertOK()
		d.send(id)

		list := visitor.GetJSON("/widget/v1/conversations").AssertOK()
		conv := list.String("data.0.id")
		require.NotEmpty(t, conv)
		visitor.GetJSON("/widget/v1/conversations/"+conv+"/messages").AssertOK().
			AssertJSONPath("data.0.body", "Hi Ada, want a hand getting set up?")

		visitor.PostJSON("/widget/v1/conversations/"+conv+"/messages", testkit.JSON{
			"body": "Yes please, Thursday works.", "client_id": ids.New().String(),
		}).AssertStatus(201)
		d.drain()

		r := d.recipients(id)[0]
		assert.NotNil(t, r.RepliedAt)
		d.report(id).AssertJSONPath("data.counts.replied", float64(1))
	})

	t.Run("leaves out people who never used the Messenger", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.contact(unique("quiet"), "Quiet", "pro")

		id := d.owner.PostJSON("/api/v1/broadcasts", testkit.JSON{"name": "Post", "channel": "post"}).
			AssertCreated().String("data.id")
		d.owner.PatchJSON("/api/v1/broadcasts/"+id, testkit.JSON{
			"subject": "New pricing",
			"body":    "Nothing changes for you until January.",
			"rules": testkit.JSON{
				"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"},
			},
			"sender_id": models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID),
		}).AssertOK()
		d.send(id)

		d.report(id).AssertJSONPath("data.reasons.0.reason", "no_messenger")
	})
}

/*
query runs the function with the workspace's queries.
*/
func query(d desk, fn func(ctx context.Context, q *models.Queries)) {
	d.tc.T.Helper()

	ws := d.tc.Workspace.Workspace.ID
	ctx := tenancy.With(d.tc.T.Context(), ws.String())
	require.NoError(d.tc.T, d.tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		fn(ctx, models.New(tx))

		return nil
	}))
}
