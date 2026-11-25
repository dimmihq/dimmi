package series_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestSeriesControlGroup(t *testing.T) {
	t.Parallel()

	t.Run("holds a share of entrants out and counts their goal beside everyone else's", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Activation", "contact", testkit.JSON{
			"entry": testkit.JSON{"event": "signed_up", "reentry": testkit.JSON{"kind": "once"}},
			"goal":  testkit.JSON{"event": "installed", "days": 7, "control_percent": 50},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "wait", "kind": "wait_for", "settings": testkit.JSON{
					"event": "invited", "amount": 3, "unit": "days",
				}},
			},
			"edges": []testkit.JSON{edge("entry", "", "wait")},
		})
		d.publish(id)

		const people = 20
		contacts := make([]string, people)
		for i := range contacts {
			contacts[i] = d.contact(unique(fmt.Sprintf("p%d", i)), "Person", testkit.JSON{})
			d.owner.PostJSON("/api/v1/contact-events", testkit.JSON{
				"name": "signed_up", "contact_id": contacts[i], "idempotency_key": "s" + contacts[i],
			}).AssertOK()
		}
		d.drain()

		report := d.report(id)
		assert.Equal(t, int64(50), num(report, "data.control.percent"))
		groups := map[string]int64{}
		for i := range 2 {
			at := fmt.Sprintf("data.control.groups.%d.", i)
			groups[report.String(at+"group")] = num(report, at+"entered")
		}
		require.Equal(t, int64(people), groups["control"]+groups["treated"])
		require.Positive(t, groups["control"])
		require.Positive(t, groups["treated"])

		for _, c := range contacts {
			d.owner.PostJSON("/api/v1/contact-events", testkit.JSON{
				"name": "installed", "contact_id": c, "idempotency_key": "i" + c,
			}).AssertOK()
		}
		d.drain()
		require.NoError(t, d.tc.App.Actions.Series.Sweep(t.Context()))
		d.drain()

		report = d.report(id)
		assert.Equal(t, int64(people), num(report, "data.goal"))
		for i := range 2 {
			group := report.String(fmt.Sprintf("data.control.groups.%d.group", i))
			assert.Equal(t, groups[group], num(report, fmt.Sprintf("data.control.groups.%d.goal", i)), group)
		}

		reasons := map[string]int64{}
		for i := range 3 {
			at := fmt.Sprintf("data.reasons.%d.", i)
			reasons[report.String(at+"reason")] = num(report, at+"count")
		}
		assert.Equal(t, groups["control"], reasons["control"])
	})

	t.Run("refuses a control group over half of the entrants", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Too many", "contact", testkit.JSON{
			"entry": testkit.JSON{"event": "signed_up", "reentry": testkit.JSON{"kind": "once"}},
			"goal":  testkit.JSON{"event": "installed", "days": 7, "control_percent": 80},
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}},
			"edges": []testkit.JSON{},
		})

		body := string(d.owner.PostJSON("/api/v1/series/"+id+"/check", testkit.JSON{}).AssertOK().Body)
		assert.Contains(t, body, `"goal_control_invalid"`)
	})
}
