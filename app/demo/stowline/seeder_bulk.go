/*
|--------------------------------------------------------------------------
| Seeding In Bulk
|--------------------------------------------------------------------------
|
| Companies, people, conversations, messages, tags, ratings and past AI
| runs go in thousands of rows per statement. COPY is refused on tables
| under row-level security for the application's role, so each batch is
| one multi-row insert inside the workspace's transaction, which works
| for every role and stays fast. Conversations go in as they stand
| today, so the event log records each one's creation and close at their
| own times, and the reports builder then works out their facts from
| the same rows a live inbox leaves.
|
*/

package stowline

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
maxParams is how many values one statement may carry.
*/
const maxParams = 60_000

/*
rowSet is rows for one table's columns.
*/
type rowSet struct {
	name string
	cols []string
	rows [][]any
}

/*
insertMany inserts the rows, as many per statement as the parameter
limit allows.
*/
func insertMany(ctx context.Context, tx *database.Tx, set rowSet) error {
	per := max(1, maxParams/len(set.cols))
	for start := 0; start < len(set.rows); start += per {
		batch := set.rows[start:min(start+per, len(set.rows))]
		var b strings.Builder
		b.WriteString("INSERT INTO " + set.name + " (" + strings.Join(set.cols, ", ") + ") VALUES ")
		args := make([]any, 0, len(batch)*len(set.cols))
		for i, row := range batch {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("(")
			for j := range set.cols {
				if j > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "$%d", len(args)+j+1)
			}
			b.WriteString(")")
			args = append(args, row...)
		}
		if _, err := tx.Exec(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("insert into %s: %w", set.name, err)
		}
	}
	return nil
}

/*
stableID makes a version 7 id at the moment whose random part is the
hash of the name, so the same plan always gives the same ids.
*/
func stableID(at time.Time, name string) ids.UUID {
	h := fnv.New128a()
	_, _ = h.Write([]byte(name))
	var id ids.UUID
	copy(id[:], h.Sum(nil))
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(max(at.UnixMilli(), 0)))
	copy(id[:6], ms[2:])
	id[6] = 0x70 | id[6]&0x0f
	id[8] = 0x80 | id[8]&0x3f
	return id
}

func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

/*
people writes the companies and their people.
*/
func (r *run) people(ctx context.Context) error {
	return r.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, set := range append([]rowSet{r.companyRows()}, r.contactRows()...) {
			if err := insertMany(ctx, tx, set); err != nil {
				return err
			}
		}
		return nil
	})
}

/*
companyRows are the planned companies as rows.
*/
func (r *run) companyRows() rowSet {
	set := rowSet{name: "companies", cols: []string{
		"id", "workspace_id", "external_id", "name", "domain", "custom_attributes", "created_at", "updated_at",
	}}
	for i, c := range r.plan.Companies {
		set.rows = append(set.rows, []any{
			c.ID, r.ws.ID, fmt.Sprintf("co_%05d", 10000+i), c.Name, c.Domain,
			jsonOf(map[string]any{
				"plan": c.Plan, "seats": c.Seats, "monthly_labels": c.MonthlyLabels, "industry": c.Industry,
				"platform": c.Platform, "country": c.Country,
			}),
			c.CreatedAt, c.CreatedAt,
		})
	}
	return set
}

/*
contactRows are the planned people, their email identities and the VIP
tag on enterprise customers' people.
*/
func (r *run) contactRows() []rowSet {
	ws := r.ws.ID
	contacts := rowSet{name: "contacts", cols: []string{
		"id", "workspace_id", "kind", "external_id", "email", "email_verified", "name", "phone", "company_id",
		"custom_attributes", "locale", "timezone", "location", "first_seen_at", "last_seen_at", "created_at",
		"updated_at",
	}}
	identities := rowSet{name: "contact_identities", cols: []string{
		"id", "workspace_id", "contact_id", "type", "value", "created_at", "updated_at",
	}}
	vip := rowSet{name: "contact_tag", cols: []string{
		"workspace_id", "contact_id", "tag_id", "created_by_type", "created_by_id", "created_at",
	}}
	for i, c := range r.plan.Contacts {
		company, attrs, enterprise := r.companyOf(c)
		contacts.rows = append(contacts.rows, []any{
			c.ID, ws, map[bool]string{true: "lead", false: "user"}[c.Lead], nilIfEmpty(c.ExternalID),
			nilIfEmpty(c.Email), c.Email != "" && i%3 != 0, c.Name, nilIfEmpty(c.Phone), company, jsonOf(attrs),
			c.Locale, c.Timezone, jsonOf(map[string]string{"city": c.City, "country": c.Country}),
			c.FirstSeen, c.LastSeen, c.FirstSeen, c.LastSeen,
		})
		if c.Email != "" {
			identities.rows = append(identities.rows, []any{
				stableID(c.FirstSeen, "identity/"+c.ID.String()), ws, c.ID, "email", strings.ToLower(c.Email),
				c.FirstSeen, c.FirstSeen,
			})
		}
		if enterprise {
			vip.rows = append(vip.rows, []any{ws, c.ID, r.tags["vip"], "member", r.owner.Member.ID, c.FirstSeen})
		}
	}
	return []rowSet{contacts, identities, vip}
}

/*
companyOf is a person's company id, attributes and whether the company
is on Enterprise.
*/
func (r *run) companyOf(c Contact) (any, map[string]any, bool) {
	attrs := map[string]any{"plan": c.Plan, "role": c.Role}
	if c.Company < 0 {
		return nil, attrs, false
	}
	co := r.plan.Companies[c.Company]
	attrs["platform"] = co.Platform
	attrs["labels_last_30_days"] = co.MonthlyLabels / max(co.Seats, 1)
	return co.ID, attrs, co.Plan == "enterprise"
}

/*
conversationBatch is how many conversations one statement round writes.
*/
const conversationBatch = 5_000

/*
conversations writes every planned conversation with its messages,
tags and rating, in batches, then numbers them and builds their facts.
*/
func (r *run) conversations(ctx context.Context) error {
	all := r.plan.Conversations
	for start := 0; start < len(all); start += conversationBatch {
		end := min(start+conversationBatch, len(all))
		err := r.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			for _, set := range r.conversationRows(start, all[start:end]) {
				if err := insertMany(ctx, tx, set); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		r.s.Log("  wrote conversations %d to %d", start+1, end)
	}
	if _, err := r.s.App.DB.Exec(ctx, `UPDATE workspaces SET next_conversation_number = $2 WHERE id = $1`,
		r.ws.ID, len(all)+1); err != nil {
		return fmt.Errorf("advance conversation number: %w", err)
	}
	return r.facts(ctx)
}

/*
facts builds the reports' facts for every conversation the batches
queued, with the real builder.
*/
func (r *run) facts(ctx context.Context) error {
	far := r.s.App.Clock.Now().Add(24 * time.Hour)
	built := 0
	for {
		n, err := r.s.App.Actions.ReportMaintenance.Drain(ctx, far)
		if err != nil {
			return fmt.Errorf("build report facts: %w", err)
		}
		if n == 0 {
			break
		}
		built += n
	}
	r.s.Log("  built report facts for %d conversations", built)
	return nil
}

/*
conversationSets are the row sets one batch of conversations fills.
*/
type conversationSets struct {
	conversations, messages, tags, ratings rowSet
}

/*
conversationRows turns one batch of conversations, the first numbered
offset+1, into rows.
*/
func (r *run) conversationRows(offset int, batch []Conversation) []rowSet {
	sets := conversationSets{
		conversations: rowSet{name: "conversations", cols: conversationColumns()},
		messages: rowSet{name: "messages", cols: []string{
			"id", "workspace_id", "conversation_id", "type", "author_type", "author_id", "body", "sent_at",
			"delivery_status", "channel_connection_id", "created_at", "updated_at",
		}},
		tags: rowSet{name: "conversation_tag", cols: []string{
			"workspace_id", "conversation_id", "tag_id", "created_by_type", "created_by_id", "created_at",
		}},
		ratings: rowSet{name: "conversation_ratings", cols: []string{
			"workspace_id", "conversation_id", "contact_id", "member_id", "channel", "closed_at", "ask_at",
			"asked_at", "score", "comment", "rated_at", "created_at", "updated_at",
		}},
	}
	for i, c := range batch {
		r.conversationRow(&sets, offset+i+1, c)
	}
	return []rowSet{sets.conversations, sets.messages, sets.tags, sets.ratings}
}

/*
thread is what a conversation's messages tell its row: when the
customer last wrote, when the team first and last replied, its last
public message, how many public messages and agent replies it has.
*/
type thread struct {
	firstReply, lastReply, lastCustomer, last time.Time
	lastID                                    ids.UUID
	count, agent                              int
}

/*
conversationRow adds one conversation's rows to the sets.
*/
func (r *run) conversationRow(sets *conversationSets, number int, c Conversation) {
	contact := r.plan.Contacts[c.Contact]
	var member any
	if c.Teammate >= 0 && c.Teammate < len(r.team) {
		member = r.team[c.Teammate].Member.ID
	}
	th := r.messageRows(&sets.messages, c, member)

	var team any
	if member != nil || c.Status == "open" {
		team = r.teams[c.Team]
	}
	var handedAt, waiting any
	if c.AI == "handed_off" {
		handedAt = lastAgentMessage(c)
	}
	if c.Status == "open" && th.lastCustomer.Equal(th.last) {
		waiting = th.lastCustomer
	}
	updated := th.last
	if !c.ClosedAt.IsZero() {
		updated = c.ClosedAt
	}
	aiState := map[string]string{"": "off", "resolved": "active", "handed_off": "handed_off"}[c.AI]
	sets.conversations.rows = append(sets.conversations.rows, []any{
		c.ID, r.ws.ID, number, r.inboxes[c.Inbox], r.channels[c.Inbox][c.Channel], contact.ID, c.Subject, c.Status,
		member, team, aiState, nilIfZero(c.SnoozedUntil), c.Messages[0].At, nilIfZero(th.lastCustomer),
		nilIfZero(th.firstReply), nilIfZero(th.lastReply), th.last, nilIfZero(c.ClosedAt), th.count, waiting, updated,
		th.lastID, c.Priority, th.agent, c.Language, handedAt, nilIfEmpty(c.HandoffReason), c.CreatedAt, updated,
		jsonOf(map[string]string{"seed": Slug, "topic": c.Topic}),
	})
	r.tagRows(&sets.tags, c, member)
	if c.Rating > 0 {
		ch := map[bool]string{true: "email", false: "widget"}[c.Channel == "email"]
		sets.ratings.rows = append(sets.ratings.rows, []any{
			r.ws.ID, c.ID, contact.ID, member, ch, c.ClosedAt, c.ClosedAt, c.ClosedAt, c.Rating,
			nilIfEmpty(c.RatingComment), c.ClosedAt.Add(47 * time.Minute), c.ClosedAt, c.ClosedAt,
		})
	}
}

/*
conversationColumns are the columns a conversation is written with.
*/
func conversationColumns() []string {
	return []string{
		"id", "workspace_id", "number", "inbox_id", "channel_connection_id", "contact_id", "subject", "status",
		"assignee_member_id", "assignee_team_id", "ai_state", "snoozed_until", "first_customer_message_at",
		"last_customer_message_at", "first_reply_at", "last_reply_at", "last_message_at", "closed_at",
		"message_count", "waiting_since", "last_activity_at", "last_message_id", "priority", "ai_reply_count",
		"ai_language", "ai_handed_off_at", "ai_handoff_reason", "created_at", "updated_at", "source",
	}
}

/*
messageRows adds the conversation's messages and returns what they tell
the conversation's row.
*/
func (r *run) messageRows(set *rowSet, c Conversation, member any) thread {
	var th thread
	contact := r.plan.Contacts[c.Contact]
	byTeam := member
	if byTeam == nil {
		byTeam = r.owner.Member.ID
	}
	channel := r.channels[c.Inbox][c.Channel]
	for k, m := range c.Messages {
		id := stableID(m.At, fmt.Sprintf("%s/%d", c.ID, k))
		kind, authorType, author := "message", m.Author, byTeam
		switch m.Author {
		case "contact":
			author, th.lastCustomer = contact.ID, m.At
		case "agent":
			author = nil
			th.agent++
		case "note":
			kind, authorType = "note", "member"
		}
		var sent any
		status := "not_applicable"
		if kind == "message" {
			th.count++
			th.last, th.lastID = m.At, id
			if m.Author != "contact" {
				sent, status = m.At, "delivered"
				th.lastReply = m.At
				if th.firstReply.IsZero() {
					th.firstReply = m.At
				}
			}
		}
		set.rows = append(set.rows, []any{
			id, r.ws.ID, c.ID, kind, authorType, author, m.Body, sent, status, channel, m.At, m.At,
		})
	}
	return th
}

/*
tagRows adds the conversation's tags, by its teammate or by the agent.
*/
func (r *run) tagRows(set *rowSet, c Conversation, member any) {
	byType := "member"
	if member == nil {
		byType = "agent"
	}
	for _, name := range c.Tags {
		if tag, ok := r.tags[name]; ok {
			set.rows = append(set.rows, []any{r.ws.ID, c.ID, tag, byType, member, c.Messages[0].At.Add(time.Minute)})
		}
	}
}

/*
lastAgentMessage is when the agent last wrote in the conversation.
*/
func lastAgentMessage(c Conversation) any {
	var at any
	for _, m := range c.Messages {
		if m.Author == "agent" {
			at = m.At
		}
	}
	return at
}
