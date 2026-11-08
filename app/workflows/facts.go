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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
subject is what conditions are evaluated about: the conversation, its
contact and company with their tags, the channel, the trigger's data,
the run's variables, and the workspace's default office hours.
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
	return audience.Env{Facts: s, Now: s.now, Location: s.location}
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
	case "office_hours":
		v = s.officeField(rest)
	default:
		v = lookupMap(s.trigger, key)
	}

	return present(v)
}

/*
contactField returns one of the contact's fields.
*/
func (s *subject) contactField(name string) any {
	c := s.contact
	if key, ok := strings.CutPrefix(name, "custom."); ok {
		return customValue(c.CustomAttributes, key)
	}

	fields := map[string]any{
		"kind": c.Kind, "email": c.Email, "email_verified": c.EmailVerified, "phone": c.Phone, "name": c.Name,
		"external_id": c.ExternalID, "locale": c.Locale, "timezone": c.Timezone, "created_at": c.CreatedAt,
		"first_seen_at": c.FirstSeenAt, "last_seen_at": c.LastSeenAt, "has_company": c.CompanyID != nil,
		"tags": s.contactTags, "email_domain": emailDomain(c.Email),
	}

	if v, ok := fields[name]; ok {
		return v
	}

	return customValue(c.Location, name)
}

/*
companyField returns one of the company's fields, nothing without one.
*/
func (s *subject) companyField(name string) any {
	if s.company == nil {
		return nil
	}

	c := *s.company
	if key, ok := strings.CutPrefix(name, "custom."); ok {
		return customValue(c.CustomAttributes, key)
	}

	return map[string]any{"name": c.Name, "domain": c.Domain, "created_at": c.CreatedAt}[name]
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
present unwraps pointers and reports whether there is a value.
*/
func present(v any) (any, bool) {
	switch p := v.(type) {
	case nil:
		return nil, false
	case *string:
		if p == nil {
			return nil, false
		}

		return *p, true
	case *time.Time:
		if p == nil {
			return nil, false
		}

		return *p, true
	case []string:
		return p, true
	}

	return v, true
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
customValue reads one key of a JSON object of custom values.
*/
func customValue(raw json.RawMessage, key string) any {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}

	return values[key]
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

/*
emailDomain returns the domain of an address.
*/
func emailDomain(email *string) any {
	if email == nil {
		return nil
	}

	_, domain, ok := strings.Cut(*email, "@")
	if !ok {
		return nil
	}

	return domain
}
