/*
|--------------------------------------------------------------------------
| Condition Facts
|--------------------------------------------------------------------------
|
| Conditions read the conversation as it is when they are evaluated:
| the contact and company freshly loaded, the conversation's own state,
| the message or change that triggered the run, and the run's
| variables. Ids read as public ids, the same ones a definition names,
| so "team is team_01..." compares like for like.
|
*/

package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/eventroll"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
subject is what conditions are evaluated about: the conversation, its
contact and company with their tags, the channel, the trigger's data,
the run's variables, the workspace's default office hours, and its
saved audiences once a rule names one, and its customer ticket.
*/
type subject struct {
	conversation     models.Conversation
	contact          models.Contact
	company          *models.Company
	channel          string
	conversationTags []string
	contactTags      []string
	trigger          map[string]any
	vars             map[string]any
	now              time.Time
	location         *time.Location
	hours            *officehours.Schedule
	saved            *audiences.Resolver
	events           map[string]eventroll.Summary
	ticket           *ticketFacts
}

/*
loadSubject reads what conditions about a conversation need.
*/
func loadSubject(ctx context.Context, q *models.Queries, c models.Conversation) (*subject, error) {
	s := &subject{conversation: c, trigger: map[string]any{}, vars: map[string]any{}}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return nil, fmt.Errorf("find contact: %w", err)
	}

	s.contact = contact
	if contact.CompanyID != nil {
		company, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: c.WorkspaceID, ID: *contact.CompanyID})
		if err != nil {
			return nil, fmt.Errorf("find company: %w", err)
		}

		s.company = &company
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil {
		return nil, fmt.Errorf("find channel connection: %w", err)
	}

	s.channel = conn.Type
	if s.ticket, err = loadTicket(ctx, q, c); err != nil {
		return nil, err
	}

	err = s.loadTags(ctx, q)

	return s, err
}

/*
loadTags reads the conversation's and the contact's tags.
*/
func (s *subject) loadTags(ctx context.Context, q *models.Queries) error {
	c := s.conversation
	rows, err := q.ListConversationTags(ctx, models.ListConversationTagsParams{
		WorkspaceID: c.WorkspaceID, ConversationIds: []ids.UUID{c.ID},
	})
	if err != nil {
		return fmt.Errorf("list conversation tags: %w", err)
	}

	for _, r := range rows {
		s.conversationTags = append(s.conversationTags, models.PublicID(models.PrefixTag, r.Tag.ID))
	}

	contactRows, err := q.ListContactTags(ctx, models.ListContactTagsParams{
		WorkspaceID: c.WorkspaceID, ContactIds: []ids.UUID{c.ContactID},
	})
	if err != nil {
		return fmt.Errorf("list contact tags: %w", err)
	}

	for _, r := range contactRows {
		s.contactTags = append(s.contactTags, models.PublicID(models.PrefixTag, r.Tag.ID))
	}

	return nil
}

/*
env is the evaluation environment of the subject.
*/
func (s *subject) env() audience.Env {
	env := audience.Env{Facts: s, Now: s.now, Location: s.location}
	if s.saved != nil {
		env.Saved = s.saved.Saved
	}

	return env
}

/*
withSaved loads what a rule about to be evaluated needs beyond the
conversation, once each: the contact's event summaries, and the
workspace's saved audiences, whose rules may read events too.
*/
func (s *subject) withSaved(ctx context.Context, q *models.Queries, p *audience.Program) error {
	if s.events == nil && (p.NeedsEvents() || p.NeedsSaved()) {
		events, err := audiences.LoadEvents(ctx, q, s.conversation.WorkspaceID, []ids.UUID{s.contact.ID})
		if err != nil {
			return err
		}

		s.events = events[s.contact.ID]
		if s.events == nil {
			s.events = map[string]eventroll.Summary{}
		}
	}

	if s.saved != nil || !p.NeedsSaved() {
		return nil
	}

	r, err := audiences.LoadResolver(ctx, q, s.conversation.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load saved audiences: %w", err)
	}

	s.saved = r

	return nil
}

/*
Lookup returns the value of a field.
*/
func (s *subject) Lookup(key string) (any, bool) {
	root, rest, _ := strings.Cut(key, ".")
	var v any

	switch root {
	case "contact":
		v = s.contactField(rest)
	case "company":
		v = s.companyField(rest)
	case "conversation":
		v = s.conversationField(rest)
	case "run":
		v = lookupMap(s.vars, strings.TrimPrefix(rest, "vars."))
	case "time":
		v = s.timeField(rest)
	case "event":
		v = audiences.EventValue(s.events, rest, s.now)
	case "office_hours":
		v = s.officeField(rest)
	case "ticket":
		v = s.ticketField(rest)
	default:
		v = texts(lookupMap(s.trigger, key))
	}

	return audiences.Present(v)
}

/*
contactField returns one of the contact's fields.
*/
func (s *subject) contactField(name string) any {
	return audiences.ContactValue(s.contact, s.contactTags, name)
}

/*
companyField returns one of the company's fields, nothing without one.
*/
func (s *subject) companyField(name string) any {
	return audiences.CompanyValue(s.company, nil, name)
}

/*
conversationField returns one of the conversation's fields.
*/
func (s *subject) conversationField(name string) any {
	c := s.conversation
	waiting := any(nil)
	if c.WaitingSince != nil {
		waiting = s.now.Sub(*c.WaitingSince).Minutes()
	}

	return map[string]any{
		"channel_type": s.channel, "inbox_id": models.PublicID(models.PrefixInbox, c.InboxID), "status": c.Status,
		"priority": string(enums.ConversationPriorityOf(c.Priority)), "tags": s.conversationTags,
		"assignee_id": publicID(models.PrefixMember, c.AssigneeMemberID),
		"team_id":     publicID(models.PrefixTeam, c.AssigneeTeamID), "ai_state": c.AiState, "subject": c.Subject,
		"message_count": c.MessageCount, "waiting_minutes": waiting, "created_at": c.CreatedAt, "is_test": c.IsTest,
	}[name]
}

/*
officeField answers whether the team is in office hours now, nothing
when the workspace's hours are unknown.
*/
func (s *subject) officeField(name string) any {
	if name != "open" || s.hours == nil {
		return nil
	}

	return s.hours.IsOpen(s.now)
}

/*
timeField returns the weekday in the workspace's time zone.
*/
func (s *subject) timeField(name string) any {
	if name != "weekday" {
		return nil
	}

	loc := s.location
	if loc == nil {
		loc = time.UTC
	}

	return strings.ToLower(s.now.In(loc).Weekday().String()[:3])
}

/*
publicID returns the public id of an optional id.
*/
func publicID(prefix models.Prefix, id *ids.UUID) any {
	if id == nil {
		return nil
	}

	return models.PublicID(prefix, *id)
}

/*
lookupMap reads a dotted key from a map, falling back to the whole key.
*/
func lookupMap(values map[string]any, key string) any {
	if v, ok := values[key]; ok {
		return v
	}

	return nil
}
