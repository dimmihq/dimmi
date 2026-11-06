/*
|--------------------------------------------------------------------------
| Conversation List Queries
|--------------------------------------------------------------------------
|
| The list and its neighbors read the same query string: filter[view]
| (mine, unassigned, all, member:mem_..., team:team_..., inbox:inbox_...,
| spam, deleted, mentions), filter[status] (a comma list or any),
| filter[inbox], filter[assignee] (mem_..., me or none), filter[team]
| (team_... or none), filter[contact], filter[company], filter[channel],
| filter[channel_connection], filter[priority] (a comma list of levels),
| filter[tag], filter[tag_any], filter[created_after],
| filter[created_before], filter[waiting], filter[unread],
| filter[assignment_waiting], q and sort. A cursor carries a hash of
| the filters and sort it was made for, so a cursor taken to another
| list is refused rather than silently skipping rows.
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
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
Limits on how many values a filter takes.
*/
const (
	maxTagsAll = 5
	maxTagsAny = 20
)

/*
conversationAllow is what the conversation list accepts.
*/
func conversationAllow() pagination.Allow {
	return pagination.Allow{
		Filters: []string{
			"view", "status", "inbox", "assignee", "team", "contact", "company", "channel", "channel_connection",
			"priority", "tag", "tag_any", "created_after", "created_before", "waiting", "unread",
			"assignment_waiting", "ai",
		},
		Sorts:    []string{"last_message_at", "created_at", "waiting_since", "priority", "snoozed_until"},
		Includes: []string{"viewers"},
	}
}

/*
conversationCursor is where a page of conversations starts, the hash of
the list it belongs to, and for a saved view the hash of the version it
was made for.
*/
type conversationCursor struct {
	querybuilder.Keyset

	Hash     string `json:"h"`
	ViewHash string `json:"v,omitempty"`
}

/*
listQuery is a parsed conversation list query.
*/
type listQuery struct {
	filters     conversations.Filters
	sort        querybuilder.Sort
	defaultSort bool
	hash        string
}

/*
parseListQuery reads the filters and sort, refusing values no list can
have.
*/
func parseListQuery(registry *ids.Registry, query pagination.Query) (listQuery, error) {
	p := filterParser{ids: registry, filters: query.Filters}

	var lq listQuery

	lq.filters.Search = query.Search

	for _, parse := range []func(*conversations.Filters) error{p.statuses, p.refs, p.narrowing, p.ai, p.view} {
		if err := parse(&lq.filters); err != nil {
			return lq, err
		}
	}

	var err error
	if lq.sort, err = parseSort(query.Sorts, lq.filters); err != nil {
		return lq, err
	}

	lq.hash, lq.defaultSort = listHash(query), len(query.Sorts) == 0

	return lq, nil
}

/*
parseSort reads the order: one of the fixed orders, the snooze end only
when the list shows nothing but snoozed conversations, where it is also
the default.
*/
func parseSort(sorts []pagination.Sort, f conversations.Filters) (querybuilder.Sort, error) {
	onlySnoozed := len(f.Statuses) == 1 && f.Statuses[0] == string(enums.ConversationStatusSnoozed)

	if len(sorts) == 0 {
		if onlySnoozed {
			return querybuilder.SortSnoozeEnds, nil
		}

		return querybuilder.SortLatest, nil
	}

	names := make([]string, len(sorts))

	for i, sort := range sorts {
		names[i] = sort.Field
		if sort.Desc {
			names[i] = "-" + sort.Field
		}
	}

	sort := querybuilder.Sort(strings.Join(names, ","))
	if !slices.Contains(querybuilder.Sorts(), sort) || (sort == querybuilder.SortSnoozeEnds && !onlySnoozed) {
		return sort, pagination.ErrInvalidSort.With("parameter", string(sort))
	}

	return sort, nil
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
view reads filter[view], mine by default. A view of a team or an inbox
is that filter, unless the query names another.
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

	if named, ok := strings.CutPrefix(view, "team:"); ok {
		return p.scopedView(f, models.PrefixTeam, named, &f.TeamID)
	}

	if named, ok := strings.CutPrefix(view, "inbox:"); ok {
		return p.scopedView(f, models.PrefixInbox, named, &f.InboxID)
	}

	if strings.HasPrefix(view, string(models.PrefixConversationView)+"_") {
		id, err := p.ids.Decode(string(models.PrefixConversationView), view)
		if err != nil {
			return pagination.ErrInvalidFilter
		}

		f.View, f.ViewID = conversations.ViewCustom, id

		return nil
	}

	switch view {
	case "":
		f.View = conversations.ViewMine
	case conversations.ViewMine, conversations.ViewUnassigned, conversations.ViewAll, conversations.ViewSpam,
		conversations.ViewMentions, conversations.ViewDeleted:
		f.View = view
	default:
		return pagination.ErrInvalidFilter
	}

	return nil
}

/*
scopedView reads a view naming one team or inbox.
*/
func (p filterParser) scopedView(
	f *conversations.Filters, prefix models.Prefix, named string, target **ids.UUID,
) error {
	id, err := p.ids.Decode(string(prefix), named)
	if err != nil {
		return pagination.ErrInvalidFilter
	}

	f.View = conversations.ViewAll

	if *target == nil {
		*target = &id
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
ai reads filter[ai]: handed_off, active or any agent involvement.
*/
func (p filterParser) ai(f *conversations.Filters) error {
	for _, value := range p.filters["ai"] {
		switch value {
		case "handed_off", "active", "any":
			f.Ai = append(f.Ai, value)
		default:
			return pagination.ErrInvalidFilter
		}
	}

	return nil
}

/*
refs reads the filters naming rows.
*/
func (p filterParser) refs(f *conversations.Filters) error {
	var err error

	if f.InboxID, err = p.ref("inbox", models.PrefixInbox); err != nil {
		return err
	}

	if p.first("team") == "none" {
		f.NoTeam = true
	} else if f.TeamID, err = p.ref("team", models.PrefixTeam); err != nil {
		return err
	}

	if f.ContactID, err = p.ref("contact", models.PrefixContact); err != nil {
		return err
	}

	if f.CompanyID, err = p.ref("company", models.PrefixCompany); err != nil {
		return err
	}

	if f.Connections, err = p.refList("channel_connection", models.PrefixChannelConnection, 0); err != nil {
		return err
	}

	f.ChannelTypes = p.filters["channel"]

	if f.TagsAll, err = p.refList("tag", models.PrefixTag, maxTagsAll); err != nil {
		return err
	}

	if f.TagsAny, err = p.refList("tag_any", models.PrefixTag, maxTagsAny); err != nil {
		return err
	}

	return p.assignee(f)
}

/*
narrowing reads the filters on what a conversation carries: its
priority, age, and the yes or no filters.
*/
func (p filterParser) narrowing(f *conversations.Filters) error {
	for _, value := range p.filters["priority"] {
		level, ok := enums.ConversationPriority(value).Level()
		if !ok {
			return pagination.ErrInvalidFilter.With("parameter", "priority")
		}

		f.Priorities = append(f.Priorities, level)
	}

	var err error

	if f.CreatedAfter, err = p.moment("created_after"); err != nil {
		return err
	}

	if f.CreatedBefore, err = p.moment("created_before"); err != nil {
		return err
	}

	waiting, err := p.flag("waiting")
	if err != nil {
		return err
	}

	unread, err := p.flag("unread")
	if err != nil {
		return err
	}

	held, err := p.flag("assignment_waiting")
	if err != nil {
		return err
	}

	f.Waiting = waiting != nil && *waiting
	f.Unread = unread != nil && *unread
	f.AssignmentWaiting = held != nil && *held

	return nil
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
		f.AssigneeMe = true

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
refList reads a filter naming rows by their public ids, up to the
limit when there is one.
*/
func (p filterParser) refList(name string, prefix models.Prefix, limit int) ([]ids.UUID, error) {
	values := p.filters[name]
	if limit > 0 && len(values) > limit {
		return nil, pagination.ErrInvalidFilter.With("parameter", name)
	}

	out := make([]ids.UUID, 0, len(values))

	for _, value := range values {
		id, err := p.ids.Decode(string(prefix), value)
		if err != nil {
			return nil, pagination.ErrInvalidFilter.With("parameter", name)
		}

		out = append(out, id)
	}

	return out, nil
}

/*
moment reads a filter holding an ISO 8601 time.
*/
func (p filterParser) moment(name string) (*time.Time, error) {
	value := p.first(name)
	if value == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, pagination.ErrInvalidFilter.With("parameter", name)
	}

	return &t, nil
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
