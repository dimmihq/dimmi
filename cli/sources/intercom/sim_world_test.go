package intercom

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

/*
simPart is one part of a conversation or ticket.
*/
type simPart struct {
	id, typ, authorType, authorID string
	at                            int64
	file                          string
}

/*
simConv is a conversation or a ticket.
*/
type simConv struct {
	id, contact, company, assignee, team string
	ticket                               bool
	created, updated                     int64
	parts                                []simPart
	tags                                 []string
}

/*
simContact is a user or a lead.
*/
type simContact struct {
	id               string
	created, updated int64
	tags, companies  []string
	notes            int
}

/*
world is the simulated workspace. Every read and change goes through
its lock.
*/
type world struct {
	mu sync.Mutex
	t  *testing.T

	clock     int64
	admins    []string
	former    string
	teams     []string
	tags      []string
	companies []string
	coNotes   map[string]int
	contacts  []*simContact
	archived  *simContact
	convs     []*simConv
	articles  int

	scroll     string
	scrollUsed time.Time
	scrollSeq  int

	linkTTL   time.Duration
	convReads atomic.Int32
	onRead    func(n int32)

	requests, limited, failed atomic.Int32
	files                     atomic.Int32
}

const simFile = "file bytes of "

func newWorld(t *testing.T) *world {
	t.Helper()

	w := &world{t: t, clock: time.Now().Add(-30 * 24 * time.Hour).Unix(), coNotes: map[string]int{}, linkTTL: time.Hour}
	r := rand.New(rand.NewPCG(7, 7)) //nolint:gosec // a fixed workspace, not a secret

	for i := range 6 {
		w.admins = append(w.admins, fmt.Sprintf("a%d", i+1))
	}

	w.former = "a99"

	for i := range 3 {
		w.teams = append(w.teams, fmt.Sprintf("t%d", i+1))
	}

	for i := range 30 {
		w.tags = append(w.tags, fmt.Sprintf("tag%d", i+1))
	}

	for i := range 400 {
		id := fmt.Sprintf("co%d", i+1)
		w.companies = append(w.companies, id)

		if i%9 == 0 {
			w.coNotes[id] = 1 + i%3
		}
	}

	for i := range 3000 {
		w.contacts = append(w.contacts, w.newContact(r, i))
	}

	for i := range 1500 {
		w.convs = append(w.convs, w.newConv(r, fmt.Sprintf("%d", 10_000+i), false, 2+r.IntN(18)))
	}

	for _, i := range []int{3, 700, 1400} {
		w.convs[i] = w.newConv(r, w.convs[i].id, false, 1234)
	}

	for i := range 120 {
		w.convs = append(w.convs, w.newConv(r, fmt.Sprintf("%d", 90_000+i), true, 1+r.IntN(6)))
	}

	w.articles = 310

	// What a real workspace holds that its lists don't give: a contact
	// archived since it wrote in, a deleted team and a deleted tag still
	// on old conversations, an article by a teammate removed with no
	// trace, and company notes by a teammate removed since.
	w.archived = &simContact{id: "u-archived", created: w.tick(), tags: []string{w.tags[0]}}
	w.archived.updated = w.archived.created

	for _, i := range []int{5, 6, 1450} {
		w.convs[i].contact = w.archived.id
	}

	w.convs[7].team = "t-deleted"
	w.convs[8].tags = append(w.convs[8].tags, "tag-deleted")
	w.coNotes["co1"] = 2

	return w
}

func (w *world) tick() int64 { w.clock++; return w.clock }

/*
newContact makes the i-th contact, with tags, some notes and for some a
company.
*/
func (w *world) newContact(r *rand.Rand, i int) *simContact {
	c := &simContact{id: fmt.Sprintf("u%d", i+1), created: w.tick(), notes: 0}
	c.updated = c.created

	if i%7 == 0 {
		c.notes = 1 + i%4
	}

	n := 1 + r.IntN(3)
	if i%40 == 0 {
		n = 14 // More than the ten a contact's own list carries.
	}

	for _, k := range r.Perm(len(w.tags))[:n] {
		c.tags = append(c.tags, w.tags[k])
	}

	if i%3 == 0 {
		c.companies = []string{w.companies[r.IntN(len(w.companies))]}
	}

	return c
}

/*
newConv makes a conversation with n parts: replies, notes, closes and
assignments, some with files, some by a teammate removed since.
*/
func (w *world) newConv(r *rand.Rand, id string, ticket bool, n int) *simConv {
	c := &simConv{
		id: id, ticket: ticket, contact: w.contacts[r.IntN(len(w.contacts))].id, created: w.tick(),
		assignee: w.admins[r.IntN(len(w.admins))], team: w.teams[r.IntN(len(w.teams))],
	}

	if r.IntN(4) == 0 {
		c.company = w.companies[r.IntN(len(w.companies))]
	}

	c.tags = []string{w.tags[r.IntN(len(w.tags))]}

	for p := range n {
		w.addPart(r, c, p)
	}

	c.updated = w.clock

	return c
}

func (w *world) addPart(r *rand.Rand, c *simConv, p int) {
	sp := simPart{id: fmt.Sprintf("%s-%d", c.id, p), at: w.tick(), authorType: "admin"}
	sp.authorID = w.admins[r.IntN(len(w.admins))]

	switch k := r.IntN(10); {
	case k < 4:
		sp.typ, sp.authorType, sp.authorID = "comment", "user", c.contact
	case k < 7:
		sp.typ = "comment"
	case k == 7:
		sp.typ = "note"
	case k == 8:
		sp.typ = "close"
	default:
		sp.typ = "assignment"
	}

	if p == 1 && sp.authorType == "admin" && r.IntN(5) == 0 {
		sp.authorID = w.former
	}

	if r.IntN(12) == 0 {
		sp.file = fmt.Sprintf("f-%s.pdf", sp.id)
	}

	c.parts = append(c.parts, sp)
}

/*
change edits the workspace while it is read: new replies on some
conversations, new conversations, and contacts edited.
*/
func (w *world) change() {
	w.mu.Lock()
	defer w.mu.Unlock()

	r := rand.New(rand.NewPCG(9, 9)) //nolint:gosec // a fixed change, not a secret

	// Changes happen now, as they would in a real workspace.
	w.clock = max(w.clock, time.Now().Unix())

	for i := range 25 {
		c := w.convs[i*37%1500]
		w.addPart(r, c, len(c.parts))
		c.updated = w.clock
	}

	for range 10 {
		w.convs = append(w.convs, w.newConv(r, fmt.Sprintf("%d", 50_000+len(w.convs)), false, 3))
	}

	for i := range 15 {
		c := w.contacts[i*113%len(w.contacts)]
		c.tags = append(c.tags[:0], w.tags[i])
		c.updated = w.tick()
	}
}
