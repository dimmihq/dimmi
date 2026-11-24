/*
|--------------------------------------------------------------------------
| Planning The Workspace
|--------------------------------------------------------------------------
|
| Everything the seed writes in bulk is planned here first, in memory
| and without a database: the companies, their people, and a year of
| conversations with every message, who answered, when, how it ended
| and how the customer rated it. A plan comes from a seed and a moment,
| so the same seed planned at the same moment is the same plan, ids
| included, and tests can check the shape of the data without writing
| it.
|
| Conversations grow over the year the way a growing company's do, most
| land on weekdays in working hours, most are short and a few run to
| dozens of messages. The AI agent went live five months ago and since
| then answers most messenger and email conversations first, resolving
| the simple ones and handing the rest to the team.
|
*/

package stowline

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Size is how much a seed writes.
*/
type Size struct {
	Name          string
	Companies     int
	Contacts      int
	Conversations int
	Teammates     int
	Tickets       int
}

/*
Sizes are the sizes the seed offers, smallest first.
*/
func Sizes() []Size {
	return []Size{
		{Name: "small", Companies: 120, Contacts: 900, Conversations: 1_500, Teammates: 10, Tickets: 40},
		{Name: "medium", Companies: 600, Contacts: 5_000, Conversations: 12_000, Teammates: 22, Tickets: 160},
		{Name: "large", Companies: 2_500, Contacts: 25_000, Conversations: 60_000, Teammates: 40, Tickets: 400},
	}
}

/*
SizeNamed returns the size with the name.
*/
func SizeNamed(name string) (Size, error) {
	for _, s := range Sizes() {
		if s.Name == name {
			return s, nil
		}
	}
	return Size{}, fmt.Errorf("unknown size %q: use small, medium or large", name)
}

/*
AgentLaunch is how long ago the AI agent went live.
*/
const AgentLaunch = 150 * 24 * time.Hour

/*
History is how far back the conversations go.
*/
const History = 365 * 24 * time.Hour

/*
Plan is everything the seed writes in bulk.
*/
type Plan struct {
	Size          Size
	Now           time.Time
	Team          []Teammate
	Companies     []Company
	Contacts      []Contact
	Conversations []Conversation
}

/*
planner holds the random source and the moment a plan is made at.
*/
type planner struct {
	r      *Random
	now    time.Time
	topics []Topic
	weight int
}

/*
NewPlan plans a workspace of the size from the seed, as of now.
*/
func NewPlan(size Size, seed uint64, now time.Time) Plan {
	p := &planner{
		r:      NewRandom(seed),
		now:    now.UTC().Truncate(time.Minute),
		topics: Topics(),
	}
	for _, t := range p.topics {
		p.weight += t.Weight
	}

	plan := Plan{Size: size, Now: p.now, Team: Teammates(size.Teammates)}
	plan.Companies = p.companies(size.Companies)
	plan.Contacts = p.contacts(size.Contacts, plan.Companies)
	plan.Conversations = append(p.conversations(size.Conversations, plan), p.operatorCases(plan)...)
	return plan
}

/*
id makes a version 7 id at the moment, from the plan's random source.
*/
func (p *planner) id(at time.Time) ids.UUID {
	var id ids.UUID
	binary.BigEndian.PutUint64(id[8:], p.r.Uint64())
	binary.BigEndian.PutUint64(id[:8], p.r.Uint64())
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(max(at.UnixMilli(), 0)))
	copy(id[:6], ms[2:])
	id[6] = 0x70 | id[6]&0x0f
	id[8] = 0x80 | id[8]&0x3f
	return id
}

/*
ago is a moment the duration before now.
*/
func (p *planner) ago(d time.Duration) time.Time {
	return p.now.Add(-d)
}

func (p *planner) companies(n int) []Company {
	out := make([]Company, 0, n)
	platforms := Platforms()
	for i := range n {
		name, domain := ShopName(p.r, i)
		plan := pickPlan(p.r)
		if plan.Key == "free" {
			plan = Plans()[1+p.r.IntN(2)]
		}
		place := pickPlace(p.r)
		created := p.ago(time.Duration(30+p.r.IntN(1400)) * 24 * time.Hour)
		out = append(out, Company{
			ID: p.id(created), Name: name, Domain: domain, Plan: plan.Key,
			Industry: Industries()[p.r.IntN(len(Industries()))], Country: place.Country,
			Platform: platforms[p.r.IntN(len(platforms)-3)].Key,
			Seats:    1 + p.r.IntN(plan.Seats), MonthlyLabels: plan.Labels/4 + p.r.IntN(plan.Labels),
			CreatedAt: created,
		})
	}
	return out
}

func (p *planner) contacts(n int, companies []Company) []Contact {
	out := make([]Contact, 0, n)
	taken := map[string]bool{}
	for i := range n {
		c := p.contact(i, companies)
		if c.Email != "" && taken[c.Email] {
			at := strings.LastIndex(c.Email, "@")
			c.Email = fmt.Sprintf("%s.%d%s", c.Email[:at], i, c.Email[at:])
		}
		taken[c.Email] = true
		out = append(out, c)
	}
	return out
}

/*
contact plans the i-th person: someone at a customer company, a lead
who has not bought yet, or a one-person shop on the free plan.
*/
func (p *planner) contact(i int, companies []Company) Contact {
	place := pickPlace(p.r)
	c := Contact{
		Company: -1, Name: PersonName(p.r), Locale: place.Locale, Timezone: place.Timezone,
		Country: place.Country, City: place.City, Role: "member",
	}
	switch {
	case len(companies) > 0 && p.r.Float64() < 0.72:
		c.Company = p.r.IntN(len(companies))
		co := companies[c.Company]
		c.Plan, c.Email = co.Plan, emailOf(p.r, c.Name, co.Domain, i)
		if p.r.IntN(4) == 0 {
			c.Role = "admin"
		}
	case p.r.Float64() < 0.4:
		c.Lead, c.Plan = true, "free"
		if p.r.IntN(3) > 0 {
			c.Email = emailOf(p.r, c.Name, "mail.test", i)
		}
	default:
		c.Plan, c.Role = "free", "admin"
		c.Email = emailOf(p.r, c.Name, "mail.test", i)
	}
	if !c.Lead {
		c.ExternalID = fmt.Sprintf("acct_%06d", 100000+i)
	}
	if p.r.IntN(3) == 0 {
		c.Phone = fmt.Sprintf("+1 555 %03d %04d", p.r.IntN(1000), p.r.IntN(10000))
	}
	c.FirstSeen = p.ago(time.Duration(p.r.IntN(700*24)) * time.Hour)
	c.LastSeen = maxTime(c.FirstSeen, p.ago(time.Duration(p.r.IntN(60*24))*time.Hour))
	c.ID = p.id(c.FirstSeen)
	return c
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
