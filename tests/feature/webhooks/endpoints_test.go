package webhooks_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const hookURL = "https://hooks.example.com/dimmi"

func TestWebhookEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("adds, reads, changes and removes an endpoint, showing its secret once", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"conversation.created", "tag.*"}, "description": "CRM",
		}).AssertCreated().
			AssertJSONPath("data.object", "webhook_endpoint").
			AssertJSONPath("data.url", hookURL).
			AssertJSONPath("data.enabled", true).
			AssertJSONPath("data.health", "healthy").
			AssertJSONMissing("data.secret")
		id, secret := res.String("data.id"), res.String("meta.secret")
		require.True(t, strings.HasPrefix(id, "whk_"))
		require.NotEmpty(t, secret)
		path := "/api/v1/webhook-endpoints/" + id

		d.Owner.GetJSON("/api/v1/webhook-endpoints").AssertOK().AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", id).AssertJSONMissing("data.0.secret")
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.description", "CRM")
		d.Owner.PostJSON(path+"/secret-reveals", testkit.JSON{}).AssertOK().AssertJSONPath("data.secret", secret)

		d.Owner.PatchJSON(path, testkit.JSON{"event_types": []string{"*"}, "enabled": false}).AssertOK().
			AssertJSONPath("data.event_types.0", "*").
			AssertJSONPath("data.enabled", false).
			AssertJSONPath("data.disabled_reason", "manual")

		rotated := d.Owner.PostJSON(path+"/secret-rotations", testkit.JSON{"overlap_seconds": 3600}).AssertCreated()
		assert.NotEqual(t, secret, rotated.String("data.secret"))
		rotated.AssertJSONPath("data.previous_secret", secret)

		d.Owner.GetJSON(path+"/stats").AssertOK().AssertJSONPath("data.attempts", 0)
		d.Owner.Delete(path).AssertNoContent()
		d.Owner.GetJSON(path).AssertNotFound()
	})

	t.Run("refuses an event type the catalog does not list", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON("/api/v1/webhook-event-types").AssertOK().
			AssertJSONPath("data.0.object", "webhook_event_type").
			AssertJSONPath("data.0.type", "conversation.created").
			AssertJSONPath("data.0.object_type", "conversation")
		d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"nothing.happened"},
		}).AssertStatus(422).AssertJSONPath("code", "unknown_event_type")
	})

	t.Run("lets only managers see endpoints", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		user := d.TC.CreateUser()
		_, err := factories.Member(t.Context(), d.TC.App, factories.Membership{
			Workspace: d.TC.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		member := d.TC.In(user)

		member.GetJSON("/api/v1/webhook-endpoints").AssertForbidden()
		member.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"*"},
		}).AssertForbidden()
	})
}

func TestWebhookDeliveries(t *testing.T) {
	t.Parallel()

	t.Run("posts a subscribed event, logs it and sends it again on request", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP).Stub(hookURL, httpclient.Response{Status: 200, Body: []byte("ok")})

		id := d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"tag.*"},
		}).AssertCreated().String("data.id")

		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Billing"}).AssertCreated()
		d.TC.Drain()

		sent := fake.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, "tag.created", sent[0].Header.Get("Dimmi-Event-Type"))
		assert.NotEmpty(t, sent[0].Header.Get("Webhook-Signature"))

		var body struct {
			Type string `json:"type"`
			Data struct {
				Object struct {
					Object string `json:"object"`
					Name   string `json:"name"`
				} `json:"object"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(sent[0].Body, &body))
		assert.Equal(t, "tag.created", body.Type)
		assert.Equal(t, "tag", body.Data.Object.Object)
		assert.Equal(t, "Billing", body.Data.Object.Name)

		list := d.Owner.GetJSON("/api/v1/webhook-endpoints/"+id+"/deliveries").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.event_type", "tag.created").
			AssertJSONPath("data.0.status", "succeeded")
		delivery := "/api/v1/webhook-deliveries/" + list.String("data.0.id")

		d.Owner.GetJSON(delivery).AssertOK().
			AssertJSONPath("data.attempts.#", 1).
			AssertJSONPath("data.attempts.0.response_status", 200).
			AssertJSONPath("data.payload.type", "tag.created")
		d.Owner.PostJSON(delivery+"/redeliveries", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.attempts.#", 2)
		assert.Len(t, fake.Sent(), 2)
	})

	t.Run("carries conversations and messages in the API's shapes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)

		d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"conversation.created", "message.created"},
		}).AssertCreated()
		conv := d.Customer(t, "ada", "Refund").String("data.id")
		d.TC.Drain()

		types := map[string]map[string]any{}
		for _, req := range fake.Sent() {
			var body struct {
				Type string `json:"type"`
				Data struct {
					Object map[string]any `json:"object"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(req.Body, &body))
			types[body.Type] = body.Data.Object
		}

		require.Contains(t, types, "conversation.created")
		assert.Equal(t, conv, types["conversation.created"]["id"])
		assert.Equal(t, "Refund", types["conversation.created"]["subject"])
		require.Contains(t, types, "message.created")
		assert.Equal(t, conv, types["message.created"]["conversation_id"])
	})

	t.Run("sends a test event and reports the answer", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		httpclienttest.New(d.TC.App.HTTP).Stub(hookURL, httpclient.Response{Status: 401})

		id := d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"*"},
		}).AssertCreated().String("data.id")

		res := d.Owner.PostJSON("/api/v1/webhook-endpoints/"+id+"/tests", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.delivery.test", true).
			AssertJSONPath("data.delivery.event_type", "webhook.test").
			AssertJSONPath("data.attempt.response_status", 401)
		assert.NotEmpty(t, res.String("data.attempt.hint"))
	})

	t.Run("sends nothing for an event no endpoint subscribes to", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)

		d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": hookURL, "event_types": []string{"contact.created"},
		}).AssertCreated()
		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Billing"}).AssertCreated()
		d.TC.Drain()

		assert.Empty(t, fake.Sent())
	})
}
