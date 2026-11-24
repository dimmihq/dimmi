package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestOperatorRoutineLive(t *testing.T) {
	t.Parallel()

	t.Run("admits only teammates who may read the routine to its channel", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Watcher", "prompt": "Check."})
		channel := k.routineChannel(routine)
		user := k.d.TC.CreateUser()
		member, err := factories.Member(t.Context(), k.d.TC.App, factories.Membership{
			Workspace: k.d.TC.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		_, err = k.d.TC.Operator().Exec(k.t.Context(),
			`UPDATE workspace_members SET conversation_access = 'assigned' WHERE id = $1`, member.ID)
		require.NoError(t, err)
		url := k.d.TC.RealtimeURL()

		assert.Nil(t, k.d.TC.ListenAs(k.d.TC.Owner, url).Subscribe(channel).Error)
		assert.NotNil(t, k.d.TC.ListenAs(user, url).Subscribe(channel).Error)
	})

	t.Run("streams a run's steps and status to the routine's channel and keeps the steps", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Watcher", "prompt": "Check."})
		listener := k.d.TC.ListenAs(k.d.TC.Owner, k.d.TC.RealtimeURL())
		require.Nil(t, listener.Subscribe(k.routineChannel(routine)).Error)
		k.llm.Script(tools(call("list_recent_suggestions", map[string]any{})), reply("All fine."))

		run := k.runRoutine(routine, true)

		events := map[string][]string{}
		for {
			p := listener.Publication()
			events[p.Event] = append(events[p.Event], string(p.Data))
			if p.Event == "operator.routine_run.changed" && len(events[p.Event]) == 2 {
				break
			}
		}
		assert.Contains(t, events["operator.routine_run.changed"][0], `"status":"running"`)
		assert.Contains(t, events["operator.routine_run.changed"][1], `"status":"completed"`)
		require.Len(t, events["operator.step"], 1)
		assert.Contains(t, events["operator.step"][0], "list_recent_suggestions")
		assert.Equal(t, "list_recent_suggestions", run.String("data.steps.0.tool"))
	})
}

/*
routineChannel names the channel a routine's runs stream on.
*/
func (k *desk) routineChannel(routine string) string {
	k.t.Helper()
	id, err := k.d.TC.App.IDs.Decode(string(models.PrefixOperatorRoutine), routine)
	require.NoError(k.t, err)
	return appbroadcasting.OperatorRoutineChannel(id)
}
