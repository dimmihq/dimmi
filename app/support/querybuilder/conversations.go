/*
|--------------------------------------------------------------------------
| Conversation Lists
|--------------------------------------------------------------------------
|
| A conversation list narrows by any mix of filters and sorts by any of a
| dozen orders, and each combination must walk its own index or a busy
| workspace pays for it. A single prepared statement with optional
| clauses defeats the planner, so this is the one place SQL is put
| together: only the clauses a list needs, every value a bound parameter,
| never text the caller typed.
|
*/

package querybuilder

import (
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Fragment is a piece of predicate built elsewhere. Render receives the
next free placeholder number and returns the SQL and its values.
*/
type Fragment func(next int) (string, []any)

/*
Conversations is one list: which rows, in which order, from where.
Unread and Mentioned are judged for MemberID. Statuses empty means any.
*/
type Conversations struct {
	WorkspaceID   ids.UUID
	MemberID      ids.UUID
	Deleted       bool
	Spam          bool
	Statuses      []string
	Unassigned    bool
	AssigneeID    *ids.UUID
	TeamID        *ids.UUID
	NoTeam        bool
	InboxID       *ids.UUID
	ContactID     *ids.UUID
	CompanyID     *ids.UUID
	ConnectionIDs []ids.UUID
	ChannelTypes  []string
	Priorities    []int16
	PriorityMin   *int16
	Number        *int64
	Unread        bool
	Waiting       bool
	Mentioned     bool
	TagsAll       []ids.UUID
	TagsAny       []ids.UUID
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	TSQuery       *string
	Extra         []Fragment
	Sort          Sort
	After         *Keyset
	Reverse       bool
	Limit         int
}

/*
IDs returns the query selecting the ids of the list, in its order, and
its parameters.
*/
func (c Conversations) IDs() (string, []any) {
	b := builder{}
	where := c.where(&b)

	keys := c.Sort.keys()
	if c.Reverse {
		keys = reversed(keys)
	}

	if c.After != nil {
		where = append(where, "("+b.keyset(keys, *c.After)+")")
	}

	sql := "SELECT c.id FROM conversations c WHERE " + strings.Join(where, " AND ") + " ORDER BY " + orderBy(keys)
	if c.Limit > 0 {
		sql += " LIMIT " + b.add(int64(c.Limit))
	}

	return sql, b.args
}

/*
Count returns the query counting the list up to the cap, and its
parameters. Past the cap the database stops looking.
*/
func (c Conversations) Count(limit int) (string, []any) {
	b := builder{}
	where := c.where(&b)

	sql := "SELECT count(*) FROM (SELECT 1 FROM conversations c WHERE " + strings.Join(where, " AND ") +
		" LIMIT " + b.add(int64(limit)) + ") counted"

	return sql, b.args
}

/*
where returns the list's predicates.
*/
func (c Conversations) where(b *builder) []string {
	where := []string{"c.workspace_id = " + b.add(c.WorkspaceID)}

	if c.Deleted {
		where = append(where, "c.deleted_at IS NOT NULL")
	} else {
		where = append(where, "c.deleted_at IS NULL", fmt.Sprintf("c.spam = %t", c.Spam))
	}

	where = append(where, c.holders(b)...)
	where = append(where, c.channels(b)...)
	where = append(where, c.flags(b)...)
	where = append(where, c.text(b)...)

	for _, fragment := range c.Extra {
		sql, args := fragment(len(b.args) + 1)
		b.args = append(b.args, args...)
		where = append(where, sql)
	}

	return where
}

/*
holders returns the predicates on who holds a conversation and where it
sits.
*/
func (c Conversations) holders(b *builder) []string {
	var where []string

	switch len(c.Statuses) {
	case 0:
	case 1:
		where = append(where, "c.status = "+b.add(c.Statuses[0]))
	default:
		where = append(where, "c.status = ANY ("+b.add(c.Statuses)+"::text[])")
	}

	if c.Unassigned {
		where = append(where, "c.assignee_member_id IS NULL")
	}

	if c.NoTeam {
		where = append(where, "c.assignee_team_id IS NULL")
	}

	for _, ref := range []struct {
		column string
		id     *ids.UUID
	}{
		{"c.assignee_member_id", c.AssigneeID},
		{"c.assignee_team_id", c.TeamID},
		{"c.inbox_id", c.InboxID},
		{"c.contact_id", c.ContactID},
	} {
		if ref.id != nil {
			where = append(where, ref.column+" = "+b.add(*ref.id))
		}
	}

	if c.CompanyID != nil {
		where = append(where, "EXISTS (SELECT 1 FROM contacts ct WHERE ct.workspace_id = c.workspace_id"+
			" AND ct.id = c.contact_id AND ct.company_id = "+b.add(*c.CompanyID)+")")
	}

	return where
}

/*
channels returns the predicates on where a conversation came in.
*/
func (c Conversations) channels(b *builder) []string {
	var where []string

	if len(c.ConnectionIDs) > 0 {
		where = append(where, "c.channel_connection_id = ANY ("+b.add(c.ConnectionIDs)+"::uuid[])")
	}

	if len(c.ChannelTypes) > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM channel_connections cc WHERE cc.workspace_id = c.workspace_id"+
			" AND cc.id = c.channel_connection_id AND cc.type = ANY ("+b.add(c.ChannelTypes)+"::text[]))")
	}

	return where
}

/*
flags returns the predicates on what a conversation carries: its
priority, number, age, tags and what the member has seen.
*/
func (c Conversations) flags(b *builder) []string {
	var where []string

	if len(c.Priorities) > 0 {
		where = append(where, "c.priority = ANY ("+b.add(c.Priorities)+"::smallint[])")
	}

	if c.PriorityMin != nil {
		where = append(where, "c.priority >= "+b.add(*c.PriorityMin))
	}

	if c.Number != nil {
		where = append(where, "c.number = "+b.add(*c.Number))
	}

	if c.Waiting {
		where = append(where, "c.waiting_since IS NOT NULL")
	}

	if c.CreatedAfter != nil {
		where = append(where, "c.created_at >= "+b.add(*c.CreatedAfter))
	}

	if c.CreatedBefore != nil {
		where = append(where, "c.created_at < "+b.add(*c.CreatedBefore))
	}

	where = append(where, c.tags(b)...)

	if c.Unread {
		where = append(where, "EXISTS ("+unreadEntry(b, c.MemberID)+")")
	}

	if c.Mentioned {
		where = append(where, "EXISTS (SELECT 1 FROM message_mentions mm WHERE mm.workspace_id = c.workspace_id"+
			" AND mm.conversation_id = c.id AND mm.workspace_member_id = "+b.add(c.MemberID)+")")
	}

	return where
}

/*
tags returns the predicates on a conversation's tags: all of one list,
any of the other.
*/
func (c Conversations) tags(b *builder) []string {
	var where []string

	for _, tag := range c.TagsAll {
		where = append(where, "EXISTS (SELECT 1 FROM conversation_tag ct WHERE ct.workspace_id = c.workspace_id"+
			" AND ct.conversation_id = c.id AND ct.tag_id = "+b.add(tag)+")")
	}

	if len(c.TagsAny) > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM conversation_tag ct WHERE ct.workspace_id = c.workspace_id"+
			" AND ct.conversation_id = c.id AND ct.tag_id = ANY ("+b.add(c.TagsAny)+"::uuid[]))")
	}

	return where
}

/*
unreadEntry is the query finding something in the conversation someone
other than the member wrote after the member's read marker.
*/
func unreadEntry(b *builder, member ids.UUID) string {
	id := b.add(member) + "::uuid"

	return "SELECT 1 FROM messages m WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id" +
		" AND m.type <> 'event' AND m.deleted_at IS NULL" +
		" AND NOT (m.author_type = 'member' AND m.author_id IS NOT DISTINCT FROM " + id + ")" +
		" AND m.id > coalesce((SELECT r.last_read_message_id FROM conversation_reads r" +
		" WHERE r.workspace_id = c.workspace_id AND r.conversation_id = c.id AND r.workspace_member_id = " + id + ")," +
		" '00000000-0000-0000-0000-000000000000'::uuid)"
}

/*
text returns the predicate of a search: the subject, one of the
messages or notes, or the customer's details match. A query of only
stop words holds no terms and matches everything.
*/
func (c Conversations) text(b *builder) []string {
	if c.TSQuery == nil {
		return nil
	}

	query := b.add(*c.TSQuery) + "::text"
	search := "to_tsquery('dimmi_search', " + query + ")"

	return []string{"(numnode(" + search + ") = 0 OR c.search @@ " + search +
		" OR c.id IN (SELECT m.conversation_id FROM messages m WHERE m.workspace_id = c.workspace_id" +
		" AND m.deleted_at IS NULL AND m.type <> 'event' AND m.search @@ " + search + ")" +
		" OR c.contact_id IN (SELECT ct.id FROM contacts ct WHERE ct.workspace_id = c.workspace_id" +
		" AND ct.deleted_at IS NULL AND ct.search @@ to_tsquery('simple', " + query + ")))"}
}

/*
builder collects a query's parameters.
*/
type builder struct {
	args []any
}

/*
add keeps the value and returns its placeholder.
*/
func (b *builder) add(value any) string {
	b.args = append(b.args, value)

	return fmt.Sprintf("$%d", len(b.args))
}
