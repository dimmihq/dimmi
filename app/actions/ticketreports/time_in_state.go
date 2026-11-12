/*
|--------------------------------------------------------------------------
| Time In State
|--------------------------------------------------------------------------
|
| How long tickets spend in each state, per type, read from their event
| log: a ticket enters a state when it is created in it or moved to it,
| and leaves it at its next move. A state the ticket is still in counts
| up to now. Time spent resolved is not work, so it is left out. A
| ticket that comes back to a state adds the visits up, so each ticket
| counts once per state. Only tickets created in the range count.
|
*/

package ticketreports

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StateTime is how long tickets of one type spent in one state: how many
tickets were in it, and the mean, median and 90th percentile of their
time there in seconds.
*/
type StateTime struct {
	TypeID   ids.UUID
	TypeName string
	StateID  ids.UUID
	State    string
	Group    string
	Tickets  int
	Mean     float64
	Median   float64
	P90      float64
}

/*
TimeInState reports the time tickets created in the range spent in each
state, per type.
*/
func (r *Reports) TimeInState(ctx context.Context, m authz.Membership, rg Range) ([]StateTime, error) {
	var out []StateTime

	err := r.read(ctx, m, rg, func(ctx context.Context, q *models.Queries, rq reports.Query) error {
		var err error
		out, err = timeInState(ctx, q, rq)

		return err
	})

	return out, err
}

/*
visit is one ticket entering a state at a time.
*/
type visit struct {
	ticket, kind, state ids.UUID
	at                  time.Time
}

/*
dwellKey names one ticket's time in one state of its type.
*/
type dwellKey struct {
	ticket, kind, state ids.UUID
}

/*
timeInState reads the log and adds the time up per type and state.
*/
func timeInState(ctx context.Context, q *models.Queries, rq reports.Query) ([]StateTime, error) {
	a := access(rq)

	rows, err := q.ListReportStateEvents(ctx, models.ListReportStateEventsParams{
		WorkspaceID: rq.Membership.Workspace.ID, CreatedFrom: rq.Start, CreatedTo: rq.End, AccessMode: a.Mode,
		AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return nil, fmt.Errorf("list ticket state events: %w", err)
	}

	n, err := loadNames(ctx, q, rq)
	if err != nil {
		return nil, err
	}

	return summarize(dwells(visits(rows), n, rq.Now), n), nil
}

/*
visits reads the state each event moved its ticket to.
*/
func visits(rows []models.ListReportStateEventsRow) []visit {
	out := make([]visit, 0, len(rows))

	for _, row := range rows {
		var data struct {
			To string `json:"to"`
		}

		if json.Unmarshal(row.Data, &data) != nil {
			continue
		}

		state, err := ids.Parse(data.To)
		if err != nil {
			continue
		}

		out = append(out, visit{ticket: row.TicketID, kind: row.TypeID, state: state, at: row.CreatedAt})
	}

	return out
}

/*
dwells adds up each ticket's time in each state that is not resolved,
the last one up to now.
*/
func dwells(vs []visit, n names, now time.Time) map[dwellKey]float64 {
	out := map[dwellKey]float64{}

	for i, v := range vs {
		if st, ok := n.states[v.state]; !ok || st.StateGroup == "resolved" {
			continue
		}

		end := now
		if i+1 < len(vs) && vs[i+1].ticket == v.ticket {
			end = vs[i+1].at
		}

		if end.Before(v.at) {
			continue
		}

		out[dwellKey{ticket: v.ticket, kind: v.kind, state: v.state}] += end.Sub(v.at).Seconds()
	}

	return out
}

/*
summarize turns the tickets' times into figures per type and state, in
the types' and states' order.
*/
func summarize(d map[dwellKey]float64, n names) []StateTime {
	type pair struct{ kind, state ids.UUID }

	samples := map[pair][]float64{}
	for k, secs := range d {
		p := pair{kind: k.kind, state: k.state}
		samples[p] = append(samples[p], secs)
	}

	out := make([]StateTime, 0, len(samples))
	for p, s := range samples {
		ty, st := n.types[p.kind], n.states[p.state]
		out = append(out, StateTime{
			TypeID: p.kind, TypeName: ty.Name, StateID: p.state, State: st.Name, Group: st.StateGroup,
			Tickets: len(s), Mean: mean(s), Median: percentile(s, 0.5), P90: percentile(s, 0.9),
		})
	}

	slices.SortFunc(out, func(a, b StateTime) int {
		ta, tb := n.types[a.TypeID], n.types[b.TypeID]
		if ta.Position != tb.Position {
			return int(ta.Position - tb.Position)
		}

		if a.TypeName != b.TypeName {
			return cmp.Compare(a.TypeName, b.TypeName)
		}

		return int(n.states[a.StateID].Position - n.states[b.StateID].Position)
	})

	return out
}

/*
mean returns the average of the samples.
*/
func mean(s []float64) float64 {
	total := 0.0
	for _, v := range s {
		total += v
	}

	return math.Round(total / float64(len(s)))
}

/*
percentile returns the sample at the share of the sorted samples, the
nearest rank.
*/
func percentile(s []float64, p float64) float64 {
	sorted := slices.Clone(s)
	slices.Sort(sorted)

	i := int(math.Ceil(p*float64(len(sorted)))) - 1

	return math.Round(sorted[max(i, 0)])
}
