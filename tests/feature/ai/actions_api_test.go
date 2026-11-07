package ai_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestAiActionDefinitions(t *testing.T) {
	t.Parallel()

	t.Run("creates a connection whose secret is never shown back", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		res := a.d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "Shop", "kind": "http", "base_url": shopURL + "/", "auth_type": "bearer", "token": shopSeal,
		}).AssertCreated()

		res.AssertJSONPath("data.base_url", shopURL).AssertJSONPath("data.has_secret", true).
			AssertJSONPath("data.secret_hint", "…4f2a").AssertJSONPath("data.status", "active")
		assert.NotContains(t, string(res.Body), shopSeal)
		id := res.String("data.id")
		assert.True(t, strings.HasPrefix(id, "acn_"))

		listed := a.d.Owner.GetJSON("/api/v1/ai/action-connections").AssertOK()
		assert.NotContains(t, string(listed.Body), shopSeal)
		rotated := "sk_live_rotated_9z9z"
		a.d.Owner.PatchJSON("/api/v1/ai/action-connections/"+id, testkit.JSON{"token": rotated}).
			AssertOK().AssertJSONPath("data.secret_hint", "…9z9z")
	})

	t.Run("refuses a plain http base url and a private address", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		for _, url := range []string{"http://shop.example.test", "https://127.0.0.1/api", "https://user:pw@x.test"} {
			res := a.d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
				"name": "Bad", "kind": "http", "base_url": url,
			})
			assert.Equal(t, fhttp.StatusUnprocessableEntity, res.Status, url)
		}
	})

	t.Run("drafts, publishes and versions an action", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		conn := a.connection()
		body := orderLookup()
		body["connection_id"] = conn
		res := a.d.Owner.PostJSON("/api/v1/ai/actions", body).AssertCreated()
		res.AssertJSONPath("data.status", "draft").AssertJSONPath("data.side_effect", "read").
			AssertJSONPath("data.confirmation", "none").AssertJSONPath("data.has_unpublished_changes", true)
		id := res.String("data.id")

		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{"note": "first"}).
			AssertCreated().AssertJSONPath("data.number", float64(1))
		a.d.Owner.GetJSON("/api/v1/ai/actions/"+id).AssertOK().AssertJSONPath("data.status", "published").
			AssertJSONPath("data.has_unpublished_changes", false).
			AssertJSONPath("data.published_version.number", float64(1))
		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "nothing_to_publish")

		a.d.Owner.PatchJSON("/api/v1/ai/actions/"+id, testkit.JSON{"title": "Check an order"}).AssertOK().
			AssertJSONPath("data.has_unpublished_changes", true)
		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.number", float64(2))
		versions := a.d.Owner.GetJSON("/api/v1/ai/actions/"+id+"/versions").AssertOK().
			AssertJSONPath("data.#", float64(2)).AssertJSONPath("data.1.note", "first")
		assert.NotEmpty(t, versions.String("data.0.published_by.name"))
		a.d.Owner.PatchJSON("/api/v1/ai/actions/"+id, testkit.JSON{"restore_version_id": versions.String("data.1.id")}).
			AssertOK().AssertJSONPath("data.title", "Check order")
		a.d.Owner.PatchJSON("/api/v1/ai/actions/"+id, testkit.JSON{"status": "disabled"}).AssertOK().
			AssertJSONPath("data.status", "disabled")
	})

	t.Run("defaults a write to asking the customer to confirm", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		body := refund("customer", "none")
		delete(body, "confirmation")
		body["connection_id"] = a.connection()
		a.d.Owner.PostJSON("/api/v1/ai/actions", body).AssertCreated().
			AssertJSONPath("data.side_effect", "write").AssertJSONPath("data.confirmation", "customer")
	})

	t.Run("refuses broken templates, reserved names and reads that wait", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		conn := a.connection()
		cases := map[string]testkit.JSON{
			"action_template_invalid":   {"path_template": "/orders/{input.missing}"},
			"action_name_reserved":      {"name": "handoff"},
			"read_needs_no_agreement":   {"approval": "teammate"},
			"action_schema_unsupported": {"input_schema": testkit.JSON{"type": "object", "oneOf": []any{}}},
		}
		for code, change := range cases {
			body := orderLookup()
			body["connection_id"] = conn
			maps.Copy(body, change)
			a.d.Owner.PostJSON("/api/v1/ai/actions", body).AssertStatus(fhttp.StatusUnprocessableEntity).
				AssertJSONPath("code", code)
		}
	})

	t.Run("keeps changes to the people who manage actions and the reading to members", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		conn := a.connection()
		member, _ := a.d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/ai/action-connections").AssertOK()
		member.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "Mine", "kind": "http", "base_url": shopURL,
		}).AssertForbidden()
		body := orderLookup()
		body["connection_id"] = conn
		member.PostJSON("/api/v1/ai/actions", body).AssertForbidden()
	})

	t.Run("hides another workspace's connections", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		other := newActionDesk(t, nil)
		conn := other.connection()

		a.d.Owner.GetJSON("/api/v1/ai/action-connections/" + conn).AssertNotFound()
	})

	t.Run("will not delete a connection actions still use", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		conn := a.connection()
		a.action(conn, orderLookup())

		a.d.Owner.GetJSON("/api/v1/ai/action-connections").AssertOK().AssertJSONPath("data.0.actions_count", float64(1))
		a.d.Owner.Delete("/api/v1/ai/action-connections/"+conn).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "connection_in_use")
		a.d.Owner.Delete("/api/v1/ai/action-connections/" + conn + "?force=true").AssertNoContent()
		a.d.Owner.GetJSON("/api/v1/ai/action-connections/" + conn).AssertNotFound()
		a.d.Owner.GetJSON("/api/v1/ai/actions").AssertOK().AssertJSONPath("data.#", float64(0))
	})
}

func TestAiActionTests(t *testing.T) {
	t.Parallel()

	t.Run("renders a dry run without sending and masks the secret", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		id := a.action(a.connection(), orderLookup())

		res := a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/tests", testkit.JSON{
			"input": testkit.JSON{"order_id": "1042"}, "contact": testkit.JSON{"external_id": "user_42"},
		}).AssertCreated()

		res.AssertJSONPath("data.status", "rendered").
			AssertJSONPath("data.request.url", shopURL+"/orders/1042?customer=user_42").
			AssertJSONPath("data.request.headers.Authorization", "[secret]")
		assert.NotContains(t, string(res.Body), shopSeal)
		assert.Empty(t, a.api.Sent())
	})

	t.Run("runs a live read and shows what the model would read", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{
			Status: 200, Body: []byte(`{"status":"shipped","eta":"Thursday","secret_note":"internal"}`),
		})
		id := a.action(a.connection(), orderLookup())

		res := a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/tests", testkit.JSON{
			"mode": "live", "input": testkit.JSON{"order_id": "1042"}, "contact": testkit.JSON{"external_id": "u_1"},
		}).AssertCreated()

		res.AssertJSONPath("data.status", "succeeded").AssertJSONPath("data.result.status", "shipped")
		res.AssertJSONMissing("data.result.secret_note")
		require.Len(t, a.sentTo("GET", "/api/orders/1042"), 1)
		assert.Equal(t, "Bearer "+shopSeal, a.sentTo("GET", "/api/orders/1042")[0].Header.Get("Authorization"))
	})

	t.Run("refuses a live write without the explicit yes", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		id := a.action(a.connection(), refund("customer", "none"))

		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/tests", testkit.JSON{
			"mode": "live", "input": testkit.JSON{"order_id": "1", "amount_cents": 100},
			"contact": testkit.JSON{"external_id": "u_1"},
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "side_effects_not_confirmed")
		assert.Empty(t, a.api.Sent())
	})

	t.Run("reports arguments that do not fit the schema", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		id := a.action(a.connection(), orderLookup())

		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/tests", testkit.JSON{
			"input": testkit.JSON{"order_id": "../admin"}, "contact": testkit.JSON{"external_id": "u_1"},
		}).AssertCreated().AssertJSONPath("data.status", "failed").
			AssertJSONPath("data.error_code", "invalid_arguments")
	})

	t.Run("probes a connection and records a refused credential", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("GET "+shopURL+"/health", httpclient.Response{Status: 401})
		conn := a.connection()

		a.d.Owner.PostJSON("/api/v1/ai/action-connections/"+conn+"/tests", testkit.JSON{"path": "/health"}).
			AssertCreated().AssertJSONPath("data.status", "error").AssertJSONPath("data.status_reason", "auth")
	})
}
