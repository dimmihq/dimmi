/*
|--------------------------------------------------------------------------
| Conversation List Queries
|--------------------------------------------------------------------------
|
| The list and its neighbors read the same query string: filter[view]
| (mine, unassigned, all, member:mem_..., spam, mentions),
| filter[status] (a comma list or any), filter[inbox],
| filter[assignee] (mem_..., me or none), filter[team],
| filter[contact], filter[company], filter[priority], filter[unread], filter[tag] (has
| all of up to 5 tags), filter[tag_any] (has one of up to 20), q, and sort
| (-last_message_at by default, last_message_at, -created_at,
| created_at). A cursor carries a hash of the filters and sort it was
| made for, so a cursor taken to another list is refused rather than
| silently skipping rows.
|
*/

package v1

import (
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
conversationAllow is what the conversation list accepts.
*/
func conversationAllow() pagination.Allow {
	return pagination.Allow{
		Filters: []string{
			"view", "status", "inbox", "assignee", "team", "contact", "priority", "unread", "tag", "tag_any",
			"company",
		},
		Sorts: []string{"last_message_at", "created_at"},
	}
}

/*
conversationCursor is where a page of conversations starts, and the
hash of the list it belongs to.
*/
type conversationCursor struct {
	At   time.Time `json:"t"`
	ID   ids.UUID  `json:"i"`
	Hash string    `json:"h"`
}

/*
listQuery is a parsed conversation list query.
*/
type listQuery struct {
	filters conversations.Filters
	sort    conversations.Sort
	hash    string
}

/*
parseListQuery reads the filters and sort, refusing values no list can
have.
*/
func parseListQuery(registry *ids.Registry, query pagination.Query) (listQuery, error) {
	p := filterParser{ids: registry, filters: query.Filters}
	lq := listQuery{sort: conversations.Sort{Descending: true}}
	lq.filters.Search = query.Search

	for _, sort := range query.Sorts {
		lq.sort = conversations.Sort{Created: sort.Field == "created_at", Descending: sort.Desc}
	}

	if err := p.view(&lq.filters); err != nil {
		return lq, err
	}

	if err := p.statuses(&lq.filters); err != nil {
		return lq, err
	}

	if err := p.refs(&lq.filters); err != nil {
		return lq, err
	}

	lq.hash = listHash(query)

	return lq, nil
}

/*
filterParser reads the filters of one query.
*/
type filterParser struct {
	ids     *ids.Registry
	filters map[string][]string
}

/*
first returns the filter's first value, empty when not sent.
*/
func (p filterParser) first(name string) string {
	if values := p.filters[name]; len(values) > 0 {
		return values[0]
	}

	return ""
}

/*
view reads filter[view], mine by default.
*/
func (p filterParser) view(f *conversations.Filters) error {
	view := p.first("view")

	if member, ok := strings.CutPrefix(view, "member:"); ok {
		id, err := p.ids.Decode(string(models.PrefixMember), member)
		if err != nil {
			return pagination.ErrInvalidFilter
		}

		f.View, f.ViewMember = conversations.ViewMember, id

		return nil
	}

	switch view {
	case "":
		f.View = conversations.ViewMine
	case conversations.ViewMine, conversations.ViewUnassigned, conversations.ViewAll, conversations.ViewSpam,
		conversations.ViewMentions:
		f.View = view
	default:
		return pagination.ErrInvalidFilter
	}

	return nil
}

/*
statuses reads filter[status], a comma list or any.
*/
func (p filterParser) statuses(f *conversations.Filters) error {
	for _, status := range p.filters["status"] {
		switch status {
		case "any":
			f.AnyStatus = true
		case "open", "snoozed", "closed":
			f.Statuses = append(f.Statuses, status)
		default:
			return pagination.ErrInvalidFilter
		}
	}

	return nil
}

/*
refs reads the filters naming rows and the yes or no filters.
*/
func (p filterParser) refs(f *conversations.Filters) error {
	var err error

	if f.InboxID, err = p.ref("inbox", models.PrefixInbox); err != nil {
		return err
	}

	if f.TeamID, err = p.ref("team", models.PrefixTeam); err != nil {
		return err
	}

	if f.TagIDs, err = p.tags("tag", maxTagFilter); err != nil {
		return err
	}

	if f.AnyTagIDs, err = p.tags("tag_any", maxAnyTagFilter); err != nil {
		return err
	}

	if f.ContactID, err = p.ref("contact", models.PrefixContact); err != nil {
		return err
	}

	if f.CompanyID, err = p.ref("company", models.PrefixCompany); err != nil {
		return err
	}

	if f.Priority, err = p.flag("priority"); err != nil {
		return err
	}

	unread, err := p.flag("unread")
	f.Unread = unread != nil && *unread

	if err != nil {
		return err
	}

	return p.assignee(f)
}

/*
tags reads a filter naming up to limit tags.
*/
func (p filterParser) tags(name string, limit int) ([]ids.UUID, error) {
	return decodeTagFilter(p.ids, p.filters[name], limit)
}

/*
assignee reads filter[assignee]: a member, none, or me, which the
list resolves to the caller.
*/
func (p filterParser) assignee(f *conversations.Filters) error {
	switch value := p.first("assignee"); value {
	case "":
		return nil
	case "none":
		f.Unassigned = true

		return nil
	case "me":
		f.View = conversations.ViewMine

		return nil
	}

	id, err := p.ref("assignee", models.PrefixMember)
	f.Assignee = id

	return err
}

/*
ref reads a filter naming one row by its public id.
*/
func (p filterParser) ref(name string, prefix models.Prefix) (*ids.UUID, error) {
	value := p.first(name)
	if value == "" {
		return nil, nil
	}

	id, err := p.ids.Decode(string(prefix), value)
	if err != nil {
		return nil, pagination.ErrInvalidFilter
	}

	return &id, nil
}

/*
flag reads a true or false filter.
*/
func (p filterParser) flag(name string) (*bool, error) {
	value := p.first(name)
	if value == "" {
		return nil, nil
	}

	b, ok := boolean(value)
	if !ok {
		return nil, pagination.ErrInvalidFilter
	}

	return &b, nil
}

/*
listHash returns a short hash of the filters, search and sort, the list
a cursor belongs to.
*/
func listHash(query pagination.Query) string {
	var b strings.Builder

	for _, name := range slices.Sorted(maps.Keys(query.Filters)) {
		b.WriteString(url.QueryEscape(name) + "=" + url.QueryEscape(strings.Join(query.Filters[name], ",")) + "&")
	}

	for _, sort := range query.Sorts {
		b.WriteString("sort=" + sort.Field)
		if sort.Desc {
			b.WriteString(":desc")
		}
	}

	b.WriteString("&q=" + query.Search)

	return hashing.SHA256Hex([]byte(b.String()))[:16]
}
