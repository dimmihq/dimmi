package workflows_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/workflowsims"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

func TestTryingAWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("plays one chat on the draft and keeps nothing", func(t *testing.T) {
		t.Parallel()
		d := simDesk(t, &tapper{tap: 1})
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		wf := create(t, d, testkit.JSON{"name": "Menu", "spec": menu(tag)})
		id, err := d.TC.App.IDs.Decode(string(models.PrefixWorkflow), wf)
		require.NoError(t, err)
		sims := d.TC.App.Actions.WorkflowSims
		owner := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"workflows:manage"})

		res, err := sims.Try(t.Context(), owner, id, workflowsims.TryInput{
			Message: "Hello there", Scenario: "Has a billing question.", Persona: []byte(`{"name":"Ada Lovelace"}`),
		})
		require.NoError(t, err)

		require.NotNil(t, res.Trace)
		assert.True(t, res.Trace.Verdict.Started)
		assert.Equal(t, "p_billing", res.Trace.Steps[0].PathID)
		assert.Equal(t, "tag", res.Trace.Actions[0].Kind)
		d.Owner.GetJSON("/api/v1/workflows/"+wf+"/simulations").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON("/api/v1/conversations?limit=5").AssertOK().AssertJSONPath("data.#", 0)

		_, err = sims.Try(t.Context(), owner, id, workflowsims.TryInput{})
		require.Error(t, err, "a try needs the customer's first message")
		reader := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"workflows:read"})
		_, err = sims.Try(t.Context(), reader, id, workflowsims.TryInput{Message: "Hi"})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})
}
