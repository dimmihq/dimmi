package operator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
owner loads the owner's membership as a routine's run would.
*/
func (k *desk) owner() authz.Membership {
	k.t.Helper()
	var m authz.Membership
	k.d.TC.InWorkspace(func(ctx context.Context) error {
		var err error
		ws := k.d.TC.Workspace
		m, _, err = operator.LoadMember(ctx, models.New(k.d.TC.App.DB), ws.Workspace.ID, ws.Owner.ID)
		return err
	})
	return m
}

/*
catalog makes a catalog read as the membership and returns its text.
*/
func (k *desk) catalog(m authz.Membership, name string, args map[string]any) (string, error) {
	k.t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(k.t, err)
	return k.d.TC.App.Actions.OperatorAgent.Engine.Catalog.Call(k.t.Context(), m,
		operator.CatalogCall{Name: name, Args: raw})
}

/*
inboxConversation starts a conversation in the inbox with the subject.
*/
func (k *desk) inboxConversation(inbox, subject string) {
	k.t.Helper()
	k.d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": inbox, "author": "contact", "subject": subject, "body": "Hello.",
		"contact": testkit.JSON{"email": "ana@example.com", "name": "Ana Lima"},
	}).AssertCreated()
}

func TestOperatorRoutineCatalog(t *testing.T) {
	t.Parallel()

	t.Run("offers a routine a few catalog reads by name and answers them", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"template_key": operator.TemplateStaleness})
		k.llm.Script(tools(call("dimmi_list_help_centers", map[string]any{})), reply("Done."))

		k.runRoutine(routine, true).AssertJSONPath("data.steps.0.summary", "Read list help centers")

		sent := k.llm.Sent()
		var names []string
		for _, spec := range sent[0].Tools {
			names = append(names, spec.Name)
		}
		assert.Contains(t, names, "dimmi_list_help_centers")
		assert.Contains(t, names, "dimmi_list_ai_agents")
		assert.Contains(t, names, "dimmi_get_ai_agent_by_id")
		assert.Contains(t, names, "dimmi_list_ai_agent_simulations")
		assert.Contains(t, names, "dimmi_get_ai_agent_simulation_by_id")
		assert.Contains(t, names, "dimmi_list_ai_agent_simulation_runs")
		assert.NotContains(t, names, "dimmi_list_conversations")
		assert.Contains(t, lastResult(sent[1]).Parts[0].Text, "hc_")
	})

	t.Run("offers a chat none of them by name", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(reply("Hello."))

		k.ask(k.thread(""), "Hello").AssertOK()

		for _, spec := range k.llm.Sent()[0].Tools {
			assert.NotContains(t, spec.Name, "dimmi_")
		}
	})

	t.Run("reads the catalog with only the inboxes a routine names", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		billing := k.d.Owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Billing"}).AssertCreated().
			String("data.id")
		k.inboxConversation(k.d.InboxID(), "Support question")
		k.inboxConversation(billing, "Billing question")
		hidden, err := k.d.TC.App.IDs.Decode(string(models.PrefixInbox), billing)
		require.NoError(t, err)
		whole := k.owner()
		narrowed := whole
		narrowed.HiddenInboxes = []ids.UUID{hidden}

		all, err := k.catalog(whole, "dimmi_list_conversations", map[string]any{"filter[view]": "all"})
		require.NoError(t, err)
		some, err := k.catalog(narrowed, "dimmi_list_conversations", map[string]any{"filter[view]": "all"})
		require.NoError(t, err)

		assert.Contains(t, all, "Billing question")
		assert.Contains(t, some, "Support question")
		assert.NotContains(t, some, "Billing question")
	})

	t.Run("reads the catalog with only the permissions a routine holds", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		narrowed := k.owner()
		narrowed.Principal.Permissions = []string{"members:read"}

		_, err := k.catalog(narrowed, "dimmi_list_conversations", map[string]any{"filter[view]": "all"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})
}
