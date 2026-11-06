package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestAiProviderKeys(t *testing.T) {
	t.Parallel()

	t.Run("connects a server, validates it and lists the models it offers", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-a", "claude-b")

		res := d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Local", "provider": "openai_compatible", "base_url": "https://llm.example.com/v1",
		}).AssertCreated()

		res.AssertJSONPath("data.object", "ai_provider_key")
		res.AssertJSONPath("data.auth_type", "none")
		res.AssertJSONPath("data.status", "valid")
		res.AssertJSONPath("data.models", []string{"claude-a", "claude-b"})
		res.AssertJSONPath("data.models_count", 2)
		id := res.String("data.id")

		list := d.Owner.GetJSON("/api/v1/ai/keys").AssertOK()
		list.AssertJSONPath("data.#", 1)
		list.AssertJSONPath("data.0.id", id)
	})

	t.Run("keeps a key's secret write-only", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-a")

		res := d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Anthropic prod", "provider": "anthropic", "api_key": "sk-ant-secret-value-1234",
		}).AssertCreated()

		res.AssertJSONPath("data.key_hint", "••••1234")
		list := d.Owner.GetJSON("/api/v1/ai/keys").AssertOK()
		view := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()
		for _, body := range [][]byte{res.Body, list.Body, view.Body} {
			assert.NotContains(t, string(body), "sk-ant-secret")
			assert.NotContains(t, string(body), "credentials")
			assert.NotContains(t, string(body), "last_error")
		}
	})

	t.Run("needs a key for every provider but a keyless server", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{"name": "A", "provider": "anthropic"}).AssertStatus(422)
		d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{"name": "B", "provider": "openai_compatible"}).
			AssertStatus(422)
	})

	t.Run("tests, renames and deletes a key by its public id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-a")
		id := createKey(t, d, "Anthropic")

		d.Owner.PostJSON("/api/v1/ai/keys/"+id+"/test", nil).AssertOK().
			AssertJSONPath("data.status", "valid").AssertJSONPath("data.id", id)
		d.Owner.PatchJSON("/api/v1/ai/keys/"+id, testkit.JSON{"name": "Renamed"}).AssertOK().
			AssertJSONPath("data.name", "Renamed").AssertJSONPath("data.status", "valid")
		d.Owner.Delete("/api/v1/ai/keys/" + id).AssertNoContent()

		d.Owner.GetJSON("/api/v1/ai/keys").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.PostJSON("/api/v1/ai/keys/"+id+"/test", nil).AssertNotFound()
	})

	t.Run("records a refused key with a reason safe to show", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		refusing(d)

		res := d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Bad", "provider": "openai_compatible", "base_url": "https://llm.example.com/v1",
		}).AssertCreated()

		res.AssertJSONPath("data.status", "invalid")
		res.AssertJSONPath("data.status_reason", "the provider rejected the key")
		assert.NotContains(t, string(res.Body), "internal.example.com")
		assert.NotContains(t, string(res.Body), "sk-ant-leak")
	})

	t.Run("refuses to delete a key the agent answers with", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-a")
		id := createKey(t, d, "Anthropic")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"answer_key_id": id, "answer_model": "claude-a"}).AssertOK()

		d.Owner.Delete("/api/v1/ai/keys/" + id).AssertStatus(409)
	})

	t.Run("answers not found for an id of another kind or workspace", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/ai/keys/"+ids.New().String()+"/test", nil).AssertNotFound()
		d.Owner.Delete("/api/v1/ai/keys/" + models.PublicID(models.PrefixAiProviderKey, ids.New())).AssertNotFound()
		d.Owner.PatchJSON("/api/v1/ai/keys/conv_123", testkit.JSON{"name": "x"}).AssertNotFound()
	})

	t.Run("lets members list keys and only admins manage them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-a")
		id := createKey(t, d, "Anthropic")
		member, _ := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)

		member.GetJSON("/api/v1/ai/keys").AssertOK().AssertJSONPath("data.#", 1)
		member.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Mine", "provider": "anthropic", "api_key": "sk-ant-member",
		}).AssertForbidden()
		member.PatchJSON("/api/v1/ai/keys/"+id, testkit.JSON{"name": "Mine"}).AssertForbidden()
		member.PostJSON("/api/v1/ai/keys/"+id+"/test", nil).AssertForbidden()
		member.Delete("/api/v1/ai/keys/" + id).AssertForbidden()

		admin.PostJSON("/api/v1/ai/keys/"+id+"/test", nil).AssertOK()
	})
}
