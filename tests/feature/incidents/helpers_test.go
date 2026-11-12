package incidents_test

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
filler is a scripted model: Group puts every conversation it reads into
one group, Match answers the candidates it was told to, and Summarize
writes a fixed line.
*/
type filler struct {
	mu      sync.Mutex
	group   tickets.Group
	matches []ids.UUID
	groups  int
	summary string
}

func (f *filler) Fill(context.Context, tickets.FillQuestion) (map[string]any, error) {
	return map[string]any{}, nil
}

func (f *filler) Match(_ context.Context, q tickets.MatchQuestion) ([]ids.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []ids.UUID

	for _, c := range q.Candidates {
		if slices.Contains(f.matches, c.ID) {
			out = append(out, c.ID)
		}
	}

	return out, nil
}

func (f *filler) Group(_ context.Context, q tickets.GroupQuestion) ([]tickets.Group, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.groups++
	g := f.group

	for _, it := range q.Items {
		g.IDs = append(g.IDs, it.ID)
	}

	return []tickets.Group{g}, nil
}

func (f *filler) Summarize(context.Context, tickets.SummaryQuestion) (string, error) {
	return f.summary, nil
}

func (f *filler) answer(list ...ids.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.matches = list
}

/*
desk is a workspace with a scripted model and its ticket types by name.
*/
type desk struct {
	tests.Desk
	ai    *filler
	types map[string]string
	n     int
}

func newDesk(t *testing.T) *desk {
	t.Helper()

	d := &desk{Desk: tests.NewDesk(t), types: map[string]string{}, ai: &filler{
		group:   tickets.Group{Key: "failed_sign_in", Title: "Failed sign-ins", Area: "sign-in"},
		summary: "Customers cannot export CSV files larger than 10 MB.",
	}}
	d.TC.App.Actions.Tickets.Filler = d.ai

	res := d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()
	for i := range num(res) {
		d.types[res.String("data."+strconv.Itoa(i)+".name")] = res.String("data." + strconv.Itoa(i) + ".id")
	}

	return d
}

/*
customers starts n conversations from different customers and returns
their ids.
*/
func (d *desk) customers(t *testing.T, n int, subject string) []string {
	t.Helper()

	out := make([]string, n)
	for i := range out {
		d.n++
		out[i] = d.Customer(t, "person"+strconv.Itoa(d.n), subject).String("data.id")
	}

	return out
}

/*
detect runs the spike detector once.
*/
func (d *desk) detect(t *testing.T) {
	t.Helper()

	require.NoError(t, d.TC.App.Queue.Push(t.Context(), jobs.DetectIncidentSpikesArgs{}))
	d.TC.Drain()
}

/*
uuid decodes a public id.
*/
func uuid(t *testing.T, d *desk, public string) ids.UUID {
	t.Helper()

	id, err := d.TC.App.IDs.Decode(string(models.PrefixTicket), public)
	require.NoError(t, err)

	return id
}

func num(res *testkit.Response) int {
	n, _ := strconv.Atoi(res.String("data.#"))

	return n
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
