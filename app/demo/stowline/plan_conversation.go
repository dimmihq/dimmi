/*
|--------------------------------------------------------------------------
| Planning Conversations
|--------------------------------------------------------------------------
|
| One conversation at a time: a topic, a customer, a teammate on the
| topic's team, the details of the case, the customer's opening, the
| agent's answer when it was live and the channel allowed, the back and
| forth with the team, and how it stands now, with its tags and rating.
|
*/

package stowline

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

/*
fitBeforeNow moves a conversation that would run past now back in
time, so its last message is a little before now.
*/
func (p *planner) fitBeforeNow(c *Conversation) {
	last := c.Messages[len(c.Messages)-1].At
	if !last.After(p.now) {
		return
	}
	shift := last.Sub(p.now) + time.Duration(1+p.r.IntN(30))*time.Minute
	c.CreatedAt = c.CreatedAt.Add(-shift)
	for i := range c.Messages {
		c.Messages[i].At = c.Messages[i].At.Add(-shift)
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

/*
topic draws a topic by its weight.
*/
func (p *planner) topic() Topic {
	roll := p.r.IntN(p.weight)
	for _, t := range p.topics {
		if roll < t.Weight {
			return t
		}
		roll -= t.Weight
	}
	return p.topics[0]
}

/*
startTime draws when a conversation starts: more of them recently, as
the company grew, mostly on weekdays in working hours.
*/
func (p *planner) startTime() time.Time {
	daysAgo := math.Floor(365 * (1 - math.Sqrt(p.r.Float64())))
	day := p.now.Add(-time.Duration(daysAgo) * 24 * time.Hour).Truncate(24 * time.Hour)
	if wd := day.Weekday(); (wd == time.Saturday || wd == time.Sunday) && p.r.Float64() < 0.7 {
		day = day.Add(-time.Duration(int(wd+1)%7) * 24 * time.Hour)
	}
	hour := 13 + p.r.Float64()*10
	if p.r.Float64() < 0.2 {
		hour = p.r.Float64() * 24
	}
	at := day.Add(time.Duration(hour * float64(time.Hour)))
	if at.After(p.now) {
		at = p.now.Add(-time.Duration(1+p.r.IntN(240)) * time.Minute)
	}
	return at
}

/*
length draws how many exchanges after the opening a thread has: most
are short, a long tail runs past twenty messages.
*/
func (p *planner) length() int {
	switch roll := p.r.Float64(); {
	case roll < 0.5:
		return 1
	case roll < 0.73:
		return 2
	case roll < 0.86:
		return 3 + p.r.IntN(2)
	case roll < 0.955:
		return 5 + p.r.IntN(5)
	default:
		return 10 + p.r.IntN(14)
	}
}

/*
channelFor picks where a conversation came in, by its inbox.
*/
func (p *planner) channelFor(inbox string) string {
	roll := p.r.Float64()
	switch inbox {
	case "Developers":
		if roll < 0.55 {
			return "api"
		}
		return "email"
	case "Enterprise":
		if roll < 0.5 {
			return "slack"
		}
		return "email"
	default:
		switch {
		case roll < 0.56:
			return "widget"
		case roll < 0.95:
			return "email"
		default:
			return "api"
		}
	}
}

func (p *planner) conversations(n int, plan Plan) []Conversation {
	out := make([]Conversation, 0, n)
	byTeam := map[string][]int{}
	for i, t := range plan.Team {
		byTeam[t.Team] = append(byTeam[t.Team], i)
	}
	for range n {
		out = append(out, p.conversation(plan, byTeam))
	}
	slices.SortFunc(out, func(a, b Conversation) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out
}

/*
fill swaps a text's placeholders for the case's details.
*/
func fill(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}

/*
caseOf draws the details one conversation's text is filled with.
*/
func (p *planner) caseOf(contact Contact, mate Teammate, plan Plan) map[string]string {
	carriers, platforms := Carriers(), Platforms()
	carrier := carriers[p.r.IntN(len(carriers))]
	platform := platforms[p.r.IntN(len(platforms)-3)]
	if contact.Company >= 0 {
		platform = platformByKey(plan.Companies[contact.Company].Platform)
	}
	codes := ErrorCodes()
	letters := "ABCDEFGHJKLMNPQRSTUVWXYZ"
	return map[string]string{
		"first":    strings.Fields(contact.Name)[0],
		"me":       strings.Fields(mate.Name)[0],
		"carrier":  carrier.Name,
		"platform": platform.Name,
		"order":    fmt.Sprintf("#%d", 1000+p.r.IntN(90000)),
		"tracking": fmt.Sprintf("1Z%c%c%c%03d%010d", letters[p.r.IntN(24)], letters[p.r.IntN(24)],
			letters[p.r.IntN(24)], p.r.IntN(1000), p.r.Int64N(1e10)),
		"amount":   fmt.Sprintf("$%d.%02d", 4+p.r.IntN(180), p.r.IntN(100)),
		"plan":     planName(contact.Plan),
		"code":     codes[p.r.IntN(len(codes))].Code,
		"shipment": fmt.Sprintf("%05d", p.r.IntN(100000)),
		"invoice":  fmt.Sprintf("INV-%d-%04d", 2026, p.r.IntN(10000)),
	}
}

func platformByKey(key string) Platform {
	for _, p := range Platforms() {
		if p.Key == key {
			return p
		}
	}
	return Platforms()[0]
}

func planName(key string) string {
	for _, p := range Plans() {
		if p.Key == key {
			return p.Name
		}
	}
	return "Free"
}

/*
wait draws how long a person takes to reply: seconds to hours, skewed
fast; email replies, which keep their own pace, wait more.
*/
func (p *planner) wait(channel string) time.Duration {
	exp := 4 + p.r.Float64()*6
	if channel == "email" {
		exp += 1.5
	}
	return time.Duration(30+math.Exp(exp)) * time.Second
}

/*
draft is a conversation being written: the plan, its topic, the
details its text is filled with, and the moment of its last message.
*/
type draft struct {
	p     *planner
	c     *Conversation
	t     Topic
	vars  map[string]string
	at    time.Time
	close string
}

/*
add writes the next message, the wait after the last one.
*/
func (d *draft) add(author, body string, after time.Duration) {
	d.at = d.at.Add(after)
	d.c.Messages = append(d.c.Messages, Message{Author: author, Body: body, At: d.at})
}

/*
conversation plans one conversation: who writes in about what, whether
the agent answers it, the back and forth with the team, and how it
stands now.
*/
func (p *planner) conversation(plan Plan, byTeam map[string][]int) Conversation {
	t := p.topic()
	start := p.startTime()
	contactAt := min(int(float64(len(plan.Contacts))*math.Pow(p.r.Float64(), 1.6)), len(plan.Contacts)-1)
	contact := plan.Contacts[contactAt]
	team := byTeam[t.Team]
	if len(team) == 0 {
		team = byTeam["Support"]
	}
	mateAt := team[p.r.IntN(len(team))]
	vars := p.caseOf(contact, plan.Team[mateAt], plan)

	c := Conversation{
		ID: p.id(start), Contact: contactAt, Topic: t.Key, Inbox: t.Inbox, Channel: p.channelFor(t.Inbox),
		Subject: fill(t.Subjects[p.r.IntN(len(t.Subjects))], vars), Priority: t.Priority, Team: t.Team,
		Teammate: mateAt, Tags: []string{t.Tag}, CreatedAt: start, Language: "en", Article: fill(t.Article, vars),
	}
	d := &draft{p: p, c: &c, t: t, vars: vars, at: start}
	opening := fill(t.Openings[p.r.IntN(len(t.Openings))], vars)
	d.close = fill(pick(p.r, t.Closings, "Thanks!"), vars)
	if foreign := foreignOpenings()[contact.Locale]; len(foreign) > 0 && p.r.Float64() < 0.7 {
		f := foreign[p.r.IntN(len(foreign))]
		opening, d.close, c.Language = fill(f[0], vars), f[1], contact.Locale
	}
	d.add("contact", opening, 0)

	agentOn := start.After(p.ago(AgentLaunch)) && (c.Channel == "widget" || c.Channel == "email")
	if agentOn && p.r.Float64() < 0.72 && d.agent() {
		return c
	}
	d.exchanges()
	p.fitBeforeNow(&c)
	p.status(&c, c.Messages[len(c.Messages)-1].At)
	p.settle(&c)
	return c
}

/*
agent has the agent answer first and reports whether it resolved the
conversation on its own; otherwise it hands over to the team.
*/
func (d *draft) agent() bool {
	p, t, c := d.p, d.t, d.c
	answered := len(t.AIAnswers) > 0
	if answered {
		d.add("agent", fill(t.AIAnswers[p.r.IntN(len(t.AIAnswers))], d.vars), time.Duration(4+p.r.IntN(9))*time.Second)
	} else {
		d.add("agent", "Thanks for the details. This needs someone from the team, so I've passed it on with "+
			"what you wrote. They'll reply here.", time.Duration(5+p.r.IntN(6))*time.Second)
	}
	if t.AIResolvable && answered && p.r.Float64() < 0.55 {
		c.AI, c.Teammate = "resolved", -1
		if p.r.Float64() < 0.6 {
			d.add("contact", d.close, p.wait(""))
		}
		p.fitBeforeNow(c)
		last := c.Messages[len(c.Messages)-1].At
		c.Status, c.ClosedAt = "closed", minTime(p.now, last.Add(time.Duration(10+p.r.IntN(50))*time.Minute))
		p.settle(c)
		return true
	}
	c.AI = "handed_off"
	c.HandoffReason = pick(p.r, []string{"asked_for_human", "no_answer", "low_confidence", "sensitive_topic"}, "")
	if answered {
		d.add("contact", pick(p.r, []string{
			"That doesn't help, can I talk to a person?", "I already tried that.",
			"Can someone from the team look at my account?",
		}, ""), p.wait(""))
		d.add("agent", "Of course. I've passed this to the team with everything you've told me; someone will "+
			"reply here shortly.", 6*time.Second)
	}
	return false
}

/*
exchanges writes the back and forth with the team: the topic's replies
and follow-ups first, then, for long threads, the checks that go back
and forth until it is solved, and often the customer's thanks.
*/
func (d *draft) exchanges() {
	p, t, ch := d.p, d.t, d.c.Channel
	n := p.length()
	for k := range n {
		last := k == n-1
		if k >= len(t.Replies) {
			talk := smallTalk()[(k-len(t.Replies))%len(smallTalk())]
			d.add("member", talk[0], p.wait(ch))
			if !last {
				d.add("contact", talk[1], p.wait(ch))
			}
			continue
		}
		d.add("member", fill(t.Replies[k], d.vars), p.wait(ch))
		if len(t.Notes) > 0 && p.r.Float64() < 0.18 {
			d.add("note", fill(t.Notes[p.r.IntN(len(t.Notes))], d.vars), time.Duration(1+p.r.IntN(10))*time.Minute)
		}
		if !last {
			d.followUp(k)
		}
	}
	if p.r.Float64() < 0.65 {
		d.add("contact", d.close, p.wait(ch))
	}
}

/*
followUp writes the customer's answer to the team's k-th reply.
*/
func (d *draft) followUp(k int) {
	if k < len(d.t.FollowUps) {
		d.add("contact", fill(d.t.FollowUps[k], d.vars), d.p.wait(d.c.Channel))
		return
	}
	d.add("contact", pick(d.p.r, []string{
		"OK, thanks. One more question though.", "Got it. What's next?", "Thanks, checking now.",
	}, ""), d.p.wait(d.c.Channel))
}

/*
status decides how a conversation stands now: almost everything older
than a few days is closed, recent ones are a mix.
*/
func (p *planner) status(c *Conversation, last time.Time) {
	age := p.now.Sub(last)
	roll := p.r.Float64()
	switch {
	case age > 4*24*time.Hour && roll < 0.95:
		c.Status = "closed"
	case age > 4*24*time.Hour && roll < 0.97:
		c.Status = "snoozed"
	case age <= 4*24*time.Hour && roll < 0.45:
		c.Status = "closed"
	case age <= 4*24*time.Hour && roll < 0.55:
		c.Status = "snoozed"
	default:
		c.Status = "open"
	}
	switch c.Status {
	case "closed":
		c.ClosedAt = last.Add(time.Duration(5+p.r.IntN(600)) * time.Minute)
		if c.ClosedAt.After(p.now) {
			c.ClosedAt = p.now
		}
	case "snoozed":
		c.SnoozedUntil = p.now.Add(time.Duration(1+p.r.IntN(7)) * 24 * time.Hour)
	default:
		if p.r.Float64() < 0.18 {
			c.Teammate = -1
		}
	}
}

/*
settle adds the tags and rating a finished conversation ends with.
*/
func (p *planner) settle(c *Conversation) {
	if c.Priority >= 2 || p.r.Float64() < 0.03 {
		c.Tags = append(c.Tags, "urgent")
	}
	if p.r.Float64() < 0.06 {
		c.Tags = append(c.Tags, "bug")
	}
	if c.Language != "en" {
		c.Tags = append(c.Tags, "non-english")
	}
	if c.Status != "closed" || (c.Channel != "widget" && c.Channel != "email") || p.r.Float64() > 0.27 {
		return
	}
	roll := p.r.Float64()
	switch {
	case roll < 0.6:
		c.Rating = 5
	case roll < 0.8:
		c.Rating = 4
	case roll < 0.88:
		c.Rating = 3
	case roll < 0.93:
		c.Rating = 2
	default:
		c.Rating = 1
	}
	if p.r.Float64() < 0.35 {
		comments := ratingComments()[c.Rating]
		c.RatingComment = comments[p.r.IntN(len(comments))]
	}
}

/*
ratingComments are what customers write with each score.
*/
func ratingComments() map[int][]string {
	return map[int][]string{
		5: {
			"Super fast and friendly, thank you!", "Solved in minutes. Best support of any tool we use.",
			"Clear answer, exactly what I needed.", "Thanks for going the extra mile on this.",
		},
		4: {"Good help, took a little while.", "Fixed, but I had to explain twice.", "Helpful, thanks."},
		3: {"Answer was OK but slow.", "Problem solved but the process was confusing."},
		2: {"Took three days to get an answer.", "Still not sure why it happened."},
		1: {
			"Nobody answered for days and the parcel is lost.", "The bot kept sending me articles instead of a person.",
			"Charged twice and still waiting.",
		},
	}
}

/*
pick returns a random item, or the fallback for an empty list.
*/
func pick(r *Random, items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	return items[r.IntN(len(items))]
}
