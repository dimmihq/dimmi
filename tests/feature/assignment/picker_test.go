package assignment_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
squad is a team of members that is the default team of the desk's inbox.
*/
type squad struct {
	team    models.Team
	members []teammate
}

/*
crew makes a team of n members the default team of the inbox, with the
inbox's assignment settings.
*/
func (d desk) crew(t *testing.T, n int, settings map[string]any) squad {
	t.Helper()

	s := squad{members: make([]teammate, n)}
	rows := make([]models.WorkspaceMember, n)

	for i := range s.members {
		s.members[i] = d.join(t)
		rows[i] = s.members[i].member
	}

	team, err := factories.Team(t.Context(), d.tc.App, factories.TeamOf{
		Workspace: d.tc.Workspace.Workspace, Name: "Support", Members: rows,
	})
	require.NoError(t, err)

	s.team = team
	d.rule(t, &team.ID, settings)

	return s
}

/*
rule sets the inbox's default team and assignment settings.
*/
func (d desk) rule(t *testing.T, team *ids.UUID, settings map[string]any) {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"assignment": settings})
	require.NoError(t, err)

	_, err = d.tc.Operator().Exec(t.Context(), "UPDATE inboxes SET default_team_id = $1, settings = $2 WHERE id = $3",
		team, raw, d.inbox.Inbox.ID)
	require.NoError(t, err)
}

/*
holders counts the desk's open conversations each member holds, by
public member id.
*/
func (d desk) holders(t *testing.T, conversations []string) map[string]int {
	t.Helper()

	out := map[string]int{}

	for _, id := range conversations {
		holder := d.show(t, id).String("data.assignee.id")
		out[holder]++
	}

	return out
}

/*
arrivals starts n customer conversations and runs the queue.
*/
func (d desk) arrivals(t *testing.T, n int) []string {
	t.Helper()

	out := make([]string, n)
	for i := range out {
		out[i] = d.customer(t, "c"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}

	d.tc.Drain()

	return out
}

/*
away sets the member away through the API.
*/
func away(t *testing.T, m teammate) {
	t.Helper()

	m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away"}).AssertOK()
}

func TestRoundRobin(t *testing.T) {
	t.Parallel()

	t.Run("takes turns fairly over thirty arrivals across three", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 3, map[string]any{"mode": "round_robin"})

		counts := d.holders(t, d.arrivals(t, 30))

		for _, m := range s.members {
			assert.Equal(t, 10, counts[m.public()])
		}
	})

	t.Run("skips away members unless the inbox includes them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin"})
		away(t, s.members[0])

		counts := d.holders(t, d.arrivals(t, 4))
		assert.Equal(t, 4, counts[s.members[1].public()])

		d.rule(t, &s.team.ID, map[string]any{"mode": "round_robin", "include_away": true})
		counts = d.holders(t, d.arrivals(t, 4))
		assert.Equal(t, 2, counts[s.members[0].public()])
	})

	t.Run("never picks a deactivated member", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin"})
		d.owner.PatchJSON("/api/v1/members/"+s.members[0].public(), testkit.JSON{"status": "deactivated"}).AssertOK()

		counts := d.holders(t, d.arrivals(t, 3))
		assert.Equal(t, 3, counts[s.members[1].public()])
	})

	t.Run("hands fifty arrivals at once to the pool without a double pick", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 3, map[string]any{"mode": "round_robin"})
		d.rule(t, &s.team.ID, map[string]any{"mode": "manual"})
		ids := make([]string, 50)

		for i := range ids {
			ids[i] = d.customer(t, "p"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		}

		d.rule(t, &s.team.ID, map[string]any{"mode": "round_robin"})

		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Go(func() {
				_, err := d.tc.App.Actions.Assignment.Arrival.Handle(t.Context(), assignment.ArrivalInput{
					WorkspaceID: d.tc.Workspace.Workspace.ID, ConversationID: d.decode(t, id),
					Reason: assignment.ReasonArrival,
				})
				assert.NoError(t, err)
			})
		}

		wg.Wait()

		counts := d.holders(t, ids)
		for _, m := range s.members {
			assert.InDelta(t, 17, counts[m.public()], 1)
		}
	})
}

func TestBalanced(t *testing.T) {
	t.Parallel()

	t.Run("gives each arrival to whoever holds fewest, ties to the longest idle", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "manual"})
		busy, idle := s.members[0], s.members[1]
		for _, id := range d.arrivals(t, 3) {
			d.assign(t, id, testkit.JSON{"assignee_id": busy.public()})
		}

		d.rule(t, &s.team.ID, map[string]any{"mode": "balanced"})
		counts := d.holders(t, d.arrivals(t, 4))

		assert.Equal(t, 3, counts[idle.public()])
		assert.Equal(t, 1, counts[busy.public()])
	})
}

func TestCapacity(t *testing.T) {
	t.Parallel()

	t.Run("waits when everyone is full and drains the oldest when one closes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "round_robin", "default_capacity": 2})
		var held, waiting []string

		for _, id := range d.arrivals(t, 4) {
			if d.show(t, id).String("data.assignee.id") != "" {
				held = append(held, id)

				continue
			}

			d.show(t, id).AssertJSONPath("data.assignment_wait.reason", "all_at_capacity")
			waiting = append(waiting, id)
		}

		require.Len(t, held, 2)
		require.Len(t, waiting, 2)

		if d.show(t, waiting[1]).String("data.assignment_wait.since") <
			d.show(t, waiting[0]).String("data.assignment_wait.since") {
			waiting[0], waiting[1] = waiting[1], waiting[0]
		}

		d.assign(t, held[0], testkit.JSON{"status": "closed"})
		d.tc.Drain()

		d.show(t, waiting[0]).AssertJSONPath("data.assignee.id", s.members[0].public())
		d.show(t, waiting[0]).AssertJSONPath("data.assignment_wait", nil)
		d.show(t, waiting[1]).AssertJSONPath("data.assignment_wait.reason", "all_at_capacity")
	})

	t.Run("keeps a channel limit apart from the overall one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.crew(t, 1, map[string]any{"mode": "round_robin", "capacity_by_channel": map[string]int{"api": 1}})
		convs := d.arrivals(t, 2)

		reasons := []string{
			d.show(t, convs[0]).String("data.assignment_wait.reason"),
			d.show(t, convs[1]).String("data.assignment_wait.reason"),
		}
		assert.ElementsMatch(t, []string{"", "all_at_capacity"}, reasons)
	})

	t.Run("waits for someone to come back and says so once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "round_robin"})
		away(t, s.members[0])
		id := d.arrivals(t, 1)[0]

		d.show(t, id).AssertJSONPath("data.assignment_wait.reason", "no_available_member")
		require.NoError(t, d.tc.App.Actions.Assignment.DrainWorkspace(t.Context(), d.tc.Workspace.Workspace.ID))
		timeline := d.owner.GetJSON("/api/v1/conversations/" + id + "/messages").AssertOK()
		assert.Equal(t, 1, strings.Count(string(timeline.Body), "conversation.assignment_waiting"))

		s.members[0].client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online"}).AssertOK()
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee.id", s.members[0].public())
	})

	t.Run("tries the overflow team and keeps the first reason when it is empty too", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		extra := d.join(t)
		overflow, err := factories.Team(t.Context(), d.tc.App, factories.TeamOf{
			Workspace: d.tc.Workspace.Workspace, Name: "Overflow", Members: []models.WorkspaceMember{extra.member},
		})
		require.NoError(t, err)
		s := d.crew(t, 1, map[string]any{"mode": "round_robin", "overflow_team_id": overflow.ID})
		away(t, s.members[0])

		first := d.arrivals(t, 1)[0]
		d.show(t, first).AssertJSONPath("data.assignee.id", extra.public())

		away(t, extra)
		second := d.arrivals(t, 1)[0]
		d.show(t, second).AssertJSONPath("data.assignment_wait.reason", "no_available_member")
	})
}

func TestArrivalRules(t *testing.T) {
	t.Parallel()

	t.Run("assigns nobody on a manual inbox, even when handed to a team", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "manual"})
		id := d.arrivals(t, 1)[0]
		d.assign(t, id, testkit.JSON{"team_id": models.PublicID(models.PrefixTeam, s.team.ID)})
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee", nil)
	})

	t.Run("picks from a team a conversation is handed to", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "manual"})
		id := d.arrivals(t, 1)[0]
		d.rule(t, nil, map[string]any{"mode": "balanced"})

		d.assign(t, id, testkit.JSON{"team_id": models.PublicID(models.PrefixTeam, s.team.ID)})
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee.id", s.members[0].public())
	})

	t.Run("leaves one taken by hand while its pick waited in the queue", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin"})
		id := d.customer(t, "ann")
		d.assign(t, id, testkit.JSON{"assignee_id": s.members[1].public()})
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee.id", s.members[1].public())
	})

	t.Run("skips a conversation with the AI agent", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.crew(t, 1, map[string]any{"mode": "round_robin"})
		d.rule(t, nil, map[string]any{"mode": "manual"})
		id := d.customer(t, "ann")
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE conversations SET ai_state = 'active' WHERE id = $1",
			d.decode(t, id))
		require.NoError(t, err)

		_, err = d.tc.App.Actions.Assignment.Arrival.Handle(t.Context(), assignment.ArrivalInput{
			WorkspaceID: d.tc.Workspace.Workspace.ID, ConversationID: d.decode(t, id), Reason: assignment.ReasonArrival,
		})
		require.NoError(t, err)
		d.show(t, id).AssertJSONPath("data.assignee", nil)
	})

	t.Run("hands an away member's conversation to the pool when the customer writes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin"})
		id := d.customer(t, "ann")
		d.tc.Drain()
		holder := d.show(t, id).String("data.assignee.id")
		leaving, staying := s.members[0], s.members[1]
		if holder == staying.public() {
			leaving, staying = staying, leaving
		}

		leaving.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away", "away_reassign": true}).
			AssertOK()
		d.writes(t, id, "handback:1", false)
		d.tc.Drain()

		d.show(t, id).AssertJSONPath("data.assignee.id", staying.public())
	})
}
