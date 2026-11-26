/*
|--------------------------------------------------------------------------
| People, Teams, Tags And Fields
|--------------------------------------------------------------------------
|
| The records the rest name: teammates, teams, tags, custom field
| definitions, companies, contacts and the notes about them. Each is
| matched to what the workspace already has before anything is made, a
| teammate by email, a team by name, a tag by name, a field by key, a
| company by external id or domain, a contact by external id or email,
| so a workspace set up by hand, or a second migration, reuses them.
|
*/

package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/attributes"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/models"
	supportattributes "github.com/dimmihq/dimmi/app/support/attributes"
)

/*
specs returns how each kind of record is read.
*/
func specs() map[string]kindSpec {
	return map[string]kindSpec{
		KindTeammate:            decoder[Teammate](),
		KindTeam:                decoder[Team](),
		KindTag:                 decoder[Tag](),
		KindAttributeDefinition: decoder[AttributeDefinition](),
		KindCompany:             decoder[Company](),
		KindContact:             decoder[Contact](),
		KindContactNote:         decoder[ContactNote](),
		KindTicketType:          decoder[TicketType](),
		KindTicket:              decoder[Ticket](),
		KindHelpCenter:          decoder[HelpCenter](),
		KindCollection:          decoder[Collection](),
		KindArticle:             decoder[Article](),
		KindSnippet:             decoder[Snippet](),
		KindSavedReply:          decoder[SavedReply](),
		KindSchedule:            decoder[Schedule](),
		KindConversation:        decoder[Conversation](),
	}
}

/*
base returns the record's base.
*/
func (b *Base) base() Base { return *b }

/*
Teammates are matched by email to a member of the workspace, active or
not. One with no member here waits for imported teammates; until then
it is kept unsupported with its name, so its history is still shown as
written by them.
*/
func (t *Teammate) validate() error {
	t.Email = strings.TrimSpace(t.Email)
	if contacts.NormalizeEmail(&t.Email) == nil {
		return invalid("email", "A teammate has an email address.")
	}

	return oneOf("role", t.Role, "owner", "admin", "member")
}

func (t *Teammate) refs() []Ref { return nil }

func (t *Teammate) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	if out.label == "" {
		out.label = t.Email
	}

	m, err := s.q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
		WorkspaceID: s.ws(), Email: *contacts.NormalizeEmail(&t.Email),
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrNotSupportedYet
	}

	if err != nil {
		return out, fmt.Errorf("find teammate: %w", err)
	}

	out.id = m.ID

	return out, nil
}

/*
Teams are matched by name. Members are added when they are active
teammates here.
*/
func (t *Team) validate() error { return required("name", t.Name) }

func (t *Team) refs() []Ref { return refsOf(KindTeammate, t.MemberRefs...) }

func (t *Team) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	name := cut(strings.TrimSpace(t.Name), 80)

	team, err := s.q.FindLiveTeamByName(ctx, models.FindLiveTeamByNameParams{WorkspaceID: s.ws(), Name: name})

	switch {
	case err == nil:
		out.id = team.ID
	case errors.Is(err, database.ErrNotFound):
		made, err := s.kit().CreateTeam.Handle(ctx, teams.CreateTeamInput{Membership: s.runner.m, Name: name})
		if err != nil {
			return out, err
		}

		out.id, out.created = made.Team.ID, true
	default:
		return out, fmt.Errorf("find team: %w", err)
	}

	members := s.refs.ids(KindTeammate, t.MemberRefs)
	if len(members) < len(t.MemberRefs) {
		out.warn("member_refs", "teammate_unavailable", "Teammates who are not members here yet were left out.")
	}

	if len(members) == 0 {
		return out, nil
	}

	results, err := s.kit().AddTeamMembers.Handle(ctx, teams.AddTeamMembersInput{
		Membership: s.runner.m, TeamID: out.id, MemberIDs: members,
	})
	if err != nil {
		return out, err
	}

	if slices.Contains(results, teams.ResultInactive) {
		out.warn("member_refs", "teammate_inactive",
			"Teammates who are not active here were left out; add them once they are enabled.")
	}

	return out, nil
}

/*
Tags are matched by name, without regard to case.
*/
func (t *Tag) validate() error { return required("name", t.Name) }

func (t *Tag) refs() []Ref { return nil }

func (t *Tag) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	name := strings.TrimSpace(t.Name)

	found, ok, err := findTag(ctx, s, name)
	if err != nil {
		return out, err
	}

	if ok {
		out.id = found

		return out, nil
	}

	made, err := tags.Resolve(ctx, s.q, tags.Scope{WorkspaceID: s.ws(), Now: s.now}, tags.Refs{Names: []string{name}})
	if err != nil {
		return out, err
	}

	if len(made) != 1 {
		return out, invalid("name", "This tag could not be made.")
	}

	out.id, out.created = made[0], true

	return out, nil
}

/*
findTag finds the workspace's tag with the name, and reports whether
there is one.
*/
func findTag(ctx context.Context, s *step, name string) (ids.UUID, bool, error) {
	row, err := s.q.FindTagByName(ctx, models.FindTagByNameParams{WorkspaceID: s.ws(), Name: name})
	if errors.Is(err, database.ErrNotFound) {
		return ids.Nil(), false, nil
	}

	if err != nil {
		return ids.Nil(), false, fmt.Errorf("find tag: %w", err)
	}

	return row.ID, true, nil
}

/*
Field definitions are matched by key. Conversation fields wait for
conversation attributes.
*/
func (a *AttributeDefinition) validate() error {
	if err := required("object", a.Object); err != nil {
		return err
	}

	if err := oneOf("object", a.Object, "contact", "company", "conversation"); err != nil {
		return err
	}

	if err := required("key", a.Key); err != nil {
		return err
	}

	return oneOf("type", a.Type, supportattributes.Types()...)
}

func (a *AttributeDefinition) refs() []Ref { return nil }

func (a *AttributeDefinition) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: a.Label}
	if a.Object == "conversation" {
		return out, ErrNotSupportedYet
	}

	label := a.Label
	if strings.TrimSpace(label) == "" {
		label = a.Key
	}

	made, err := s.kit().Attributes.Create(ctx, attributes.CreateInput{
		Membership: s.runner.m, Object: a.Object, Key: a.Key, Label: cut(label, 80),
		Description: orNil(a.Description), DataType: a.Type, Options: a.Options,
	})
	if err == nil {
		out.id, out.created = made.ID, true

		return out, nil
	}

	if !errors.Is(err, attributes.ErrExists) {
		return out, err
	}

	defs, err := s.q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{
		WorkspaceID: s.ws(), Object: &a.Object,
	})
	if err != nil {
		return out, fmt.Errorf("list attribute definitions: %w", err)
	}

	for _, d := range defs {
		if d.Key == a.Key {
			out.id = d.ID

			return out, nil
		}
	}

	return out, attributes.ErrExists
}
