package channels_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestSlackSeries(t *testing.T) {
	t.Parallel()

	t.Run("an account series posts into the company's Slack channel and a thread reply opens a conversation",
		func(t *testing.T) {
			t.Parallel()
			sc := connectedCase(t)
			sc.setRoom(supportRoom, "shared", "every_message")
			owner := sc.ActingAsOwner()

			company := owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Globex"}).AssertCreated().
				String("data.id")
			companyID, err := sc.App.IDs.Decode(string(models.PrefixCompany), company)
			require.NoError(t, err)
			_, err = sc.Operator().Exec(t.Context(), "UPDATE channel_rooms SET company_id = $1 WHERE id = $2",
				companyID, sc.room(supportRoom).ID)
			require.NoError(t, err)

			created := owner.PostJSON("/api/v1/series", testkit.JSON{
				"name": "Renewal", "subject": "company", "spec": testkit.JSON{
					"entry": testkit.JSON{"reentry": testkit.JSON{"kind": "once"}},
					"nodes": []testkit.JSON{
						{"id": "entry", "kind": "entry"},
						{"id": "post", "kind": "slack", "settings": testkit.JSON{
							"body": "Hi {{ company.name }}, your renewal is next month.",
						}},
					},
					"edges": []testkit.JSON{{"from": "entry", "path": "next", "to": "post"}},
				},
			}).AssertCreated()
			id := created.String("data.id")
			owner.PatchJSON("/api/v1/series/"+id, testkit.JSON{
				"lock_version": 1, "name": "Renewal",
				"sender_id": models.PublicID(models.PrefixMember, sc.Workspace.Owner.ID),
			}).AssertOK()
			owner.PostJSON("/api/v1/series/"+id+"/publications", testkit.JSON{}).AssertOK()

			contact := owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ada", "email": "ada@globex.test"}).
				AssertCreated().String("data.id")
			owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"company_id": company}).AssertOK()
			owner.PostJSON("/api/v1/series/"+id+"/enrollments", testkit.JSON{"company_ids": []string{company}}).
				AssertCreated()

			for range 4 {
				sc.DrainLater()
			}

			posts := sc.slack.called("chat.postMessage")
			require.NotEmpty(t, posts)
			sent, err := url.QueryUnescape(posts[len(posts)-1].Form.Encode())
			require.NoError(t, err)
			assert.Contains(t, sent, "Hi Globex, your renewal is next month.")

			journeys := owner.GetJSON("/api/v1/series/" + id + "/enrollments").AssertOK()
			journeys.AssertJSONPath("data.0.status", "finished")

			sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1730000600.000100", "Can we talk pricing?"),
				"1730000500.000200"))
			convs := sc.conversations()
			require.Len(t, convs, 1)
			assert.Contains(t, body(sc.messages(convs[0])[0]), "your renewal is next month")
		})
}
