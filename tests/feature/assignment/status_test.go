package assignment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
poolEntry is one member of an assignment status, as the API shows it.
*/
type poolEntry struct {
	Member struct {
		ID string `json:"id"`
	} `json:"member"`
	OpenCount int     `json:"open_count"`
	Eligible  bool    `json:"eligible"`
	WhyNot    *string `json:"why_not"`
}

/*
status reads the inbox's assignment status, its pool by public member
id.
*/
func (d desk) status(t *testing.T) (*testkit.Response, map[string]poolEntry) {
	t.Helper()

	res := d.owner.GetJSON(d.inboxPath() + "/assignment-status?channel_type=api").AssertOK()

	var body struct {
		Data struct {
			Members []poolEntry `json:"members"`
		} `json:"data"`
	}
	res.Decode(&body)

	out := map[string]poolEntry{}
	for _, e := range body.Data.Members {
		out[e.Member.ID] = e
	}

	return res, out
}

/*
whyNot returns the reason in the entry, "" for none.
*/
func whyNot(e poolEntry) string {
	if e.WhyNot == nil {
		return ""
	}

	return *e.WhyNot
}

func TestAssignmentStatus(t *testing.T) {
	t.Parallel()

	t.Run("says why each member may not take the next conversation", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 4, map[string]any{"mode": "round_robin", "capacity_by_channel": map[string]int{"api": 1}})
		away(t, s.members[0])
		d.owner.PatchJSON("/api/v1/members/"+s.members[2].public(), testkit.JSON{"conversation_capacity": 1}).
			AssertOK()

		d.assign(t, d.customer(t, "ann"), testkit.JSON{"assignee_id": s.members[1].public()})
		d.assign(t, d.customer(t, "bob"), testkit.JSON{"assignee_id": s.members[2].public()})
		d.tc.Drain()

		res, pool := d.status(t)
		res.AssertJSONPath("data.mode", "round_robin").AssertJSONPath("data.channel_type", "api")
		require.Len(t, pool, 4)
		assert.Equal(t, "away", whyNot(pool[s.members[0].public()]))
		assert.Equal(t, "at_channel_capacity", whyNot(pool[s.members[1].public()]))
		assert.Equal(t, "at_capacity", whyNot(pool[s.members[2].public()]))
		assert.True(t, pool[s.members[3].public()].Eligible)
		assert.Equal(t, 1, pool[s.members[1].public()].OpenCount)
	})

	t.Run("counts the waiting conversations and lists them by filter", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 1, map[string]any{"mode": "round_robin"})
		away(t, s.members[0])
		waiting := d.arrivals(t, 2)
		d.rule(t, &s.team.ID, map[string]any{"mode": "manual"})
		held := d.customer(t, "cy")
		d.tc.Drain()

		res, _ := d.status(t)
		res.AssertJSONPath("data.waiting.count", 2)
		res.AssertJSONPath("data.waiting.reasons.no_available_member", 2)

		list := d.owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[assignment_waiting]=true").AssertOK()
		assert.ElementsMatch(t, waiting, []string{list.String("data.0.id"), list.String("data.1.id")})
		assert.NotContains(t, string(list.Body), held)

		d.owner.GetJSON("/api/v1/conversations?filter[assignment_waiting]=maybe").AssertStatus(400)
	})

	t.Run("hides the status from those who cannot read settings", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)

		m.client.GetJSON(d.inboxPath() + "/assignment-status").AssertForbidden()
		d.owner.GetJSON(d.inboxPath() + "/assignment-status?channel_type=fax").AssertStatus(422)
	})

	t.Run("forgets a deleted team's turn", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		s := d.crew(t, 2, map[string]any{"mode": "round_robin"})
		d.arrivals(t, 1)
		team := models.PublicID(models.PrefixTeam, s.team.ID)

		cursors := func() int {
			var n int
			require.NoError(t, d.tc.App.DB.QueryRow(t.Context(),
				"SELECT count(*) FROM team_assignment_cursors WHERE pool_id = $1", s.team.ID).Scan(&n))

			return n
		}
		require.Equal(t, 1, cursors())

		d.rule(t, nil, map[string]any{"mode": "manual"})
		d.owner.Delete("/api/v1/teams/" + team).AssertNoContent()

		assert.Equal(t, 0, cursors())
	})
}
