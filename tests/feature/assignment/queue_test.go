package assignment_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
waitingOf splits the conversations into those someone holds and those
waiting, in the order given.
*/
func (d desk) waitingOf(t *testing.T, conversations []string) ([]string, []string) {
	t.Helper()

	var held, waiting []string

	for _, id := range conversations {
		if d.show(t, id).String("data.assignee.id") != "" {
			held = append(held, id)

			continue
		}

		waiting = append(waiting, id)
	}

	return held, waiting
}

func TestAssignmentQueue(t *testing.T) {
	t.Parallel()

	t.Run("takes the highest priority first when the inbox says so", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{
			"mode": "round_robin", "default_capacity": 1, "queue_order": "priority_then_oldest",
		})
		held, waiting := d.waitingOf(t, d.arrivals(t, 3))
		require.Len(t, held, 1)
		require.Len(t, waiting, 2)

		urgent := waiting[1]
		d.assign(t, urgent, testkit.JSON{"priority": "urgent"})
		d.assign(t, held[0], testkit.JSON{"status": "closed"})
		d.tc.Drain()

		d.show(t, urgent).AssertJSONPath("data.assignee.id", s.members[0].public())
		d.show(t, waiting[0]).AssertJSONPath("data.assignment_wait.reason", "all_at_capacity")
	})

	t.Run("waits with nobody available when the team is empty", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.crew(t, 0, map[string]any{"mode": "balanced"})
		id := d.arrivals(t, 1)[0]

		d.show(t, id).AssertJSONPath("data.assignment_wait.reason", "no_available_member")
	})

	t.Run("lets two drains share the queue without a double pick", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin", "default_capacity": 2})
		away(t, s.members[0])
		away(t, s.members[1])
		convs := d.arrivals(t, 4)

		for _, m := range s.members {
			m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online"}).AssertOK()
		}

		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				assert.NoError(t, d.tc.App.Actions.Assignment.DrainWorkspace(t.Context(), d.tc.Workspace.Workspace.ID))
			})
		}
		wg.Wait()
		d.tc.Drain()

		counts := d.holders(t, convs)
		assert.Equal(t, 2, counts[s.members[0].public()])
		assert.Equal(t, 2, counts[s.members[1].public()])
	})

	t.Run("follows the rules of the inbox a deleted one moved its waiting conversations to", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "round_robin"})
		team := models.PublicID(models.PrefixTeam, s.team.ID)
		other := d.extraInbox(t)
		d.owner.PatchJSON("/api/v1/inboxes/"+other, testkit.JSON{
			"default_team_id": team, "settings": testkit.JSON{"assignment": testkit.JSON{"mode": "round_robin"}},
		}).AssertOK()
		away(t, s.members[0])
		id := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": other, "author": "contact", "subject": "Invoice", "body": "Wrong amount",
			"contact": testkit.JSON{"email": "di@example.com", "name": "Di"},
		}).AssertCreated().String("data.id")
		d.tc.Drain()
		d.show(t, id).AssertJSONPath("data.assignment_wait.reason", "no_available_member")

		d.rule(t, &s.team.ID, map[string]any{"mode": "round_robin", "include_away": true})
		target := models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)
		d.owner.Delete("/api/v1/inboxes/" + other + "?move_to=" + target).AssertNoContent()
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee.id", s.members[0].public())
	})
}
