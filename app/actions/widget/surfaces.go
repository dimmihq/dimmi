/*
|--------------------------------------------------------------------------
| Messenger Surfaces
|--------------------------------------------------------------------------
|
| Who sees a messenger's launcher and tabs, and who may start a
| conversation, as audience rules on the widget. An anonymous visitor's
| outcome goes in the public config; a known visitor's in their session,
| decided with their contact, company, tags and events. Starting a
| conversation is checked again when the first message arrives, since a
| browser's word is never enough. Saving checks every rule and moves the
| widget's version on, so no cached config keeps the old rules.
|
*/

package widget

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
The failures of messenger rules.
*/
var (
	ErrRulesInvalid = fault.New(fault.Invalid, "messenger_rules_invalid",
		"The messenger's rules cannot be used as they are.")
	ErrStartRefused = fault.New(fault.Forbidden, "conversation_start_restricted",
		"This messenger does not take new conversations from you.")
)

/*
Surfaces decides and saves messenger rules.
*/
type Surfaces struct {
	DB    *database.DB
	Clock clock.Clock
	Bust  *Directory
}

/*
Anonymous is what a visitor nobody knows gets, nil when the widget has
no rules.
*/
func (s *Surfaces) Anonymous(ctx context.Context, w models.WidgetSetting) (*audiences.Surface, error) {
	return s.decide(ctx, w, nil)
}

/*
For is what a known visitor gets, nil when the widget has no rules.
*/
func (s *Surfaces) For(ctx context.Context, w models.WidgetSetting, session models.WidgetSession,
) (*audiences.Surface, error) {
	return s.decide(ctx, w, &session)
}

/*
MayStart reports whether the session's visitor may start a conversation.
*/
func (s *Surfaces) MayStart(ctx context.Context, w models.WidgetSetting, session models.WidgetSession) error {
	surface, err := s.decide(ctx, w, &session)
	if err != nil || surface == nil || surface.Start {
		return err
	}

	return ErrStartRefused
}

/*
decide evaluates the widget's rules for a session's visitor, or for an
anonymous visitor when there is no session.
*/
func (s *Surfaces) decide(ctx context.Context, w models.WidgetSetting, session *models.WidgetSession,
) (*audiences.Surface, error) {
	rules, err := audiences.ParseMessenger(w.Audiences)
	if err != nil || rules.Empty() {
		return nil, err
	}

	compiled, _ := audiences.CompileMessenger(rules)
	var out audiences.Surface

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		env, err := s.env(ctx, q, w.WorkspaceID, session)
		if err != nil {
			return err
		}

		out = compiled.Decide(env)

		return nil
	})

	return &out, err
}

/*
env is what the rules read: the session's visitor, or a lead nobody
knows, with page and device left for the browser.
*/
func (s *Surfaces) env(ctx context.Context, q *models.Queries, ws ids.UUID, session *models.WidgetSession,
) (audience.Env, error) {
	resolver, err := audiences.LoadResolver(ctx, q, ws)
	if err != nil {
		return audience.Env{}, err
	}

	env := audience.Env{
		Now: s.Clock.Now(), Saved: resolver.Saved, Facts: audience.Values{"contact.kind": "lead"},
		Unknown: func(f audience.Field) bool { return f.Where == audience.Client },
	}

	if session == nil {
		return env, nil
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: session.ContactID})
	if err != nil {
		return env, fmt.Errorf("find session contact: %w", err)
	}

	people, err := audiences.LoadPeople(ctx, q, ws, []models.Contact{contact})
	if err != nil {
		return env, err
	}

	person := people[0]
	person.Now = env.Now
	person.Client = map[string]any{"session.verified": session.Kind == "identified"}
	env.Facts = person

	return env, nil
}

/*
SaveRulesInput is who changes which widget's rules, and the change: a
rule sent replaces that one, null removes it, one left out stays.
*/
type SaveRulesInput struct {
	Membership authz.Membership
	WidgetID   ids.UUID
	Rules      json.RawMessage
}

/*
SaveRules checks and stores a widget's rules.
*/
func (s *Surfaces) SaveRules(ctx context.Context, in SaveRulesInput) (models.WidgetSetting, error) {
	if !in.Membership.Can("widgets:manage") {
		return models.WidgetSetting{}, exceptions.ErrForbidden
	}

	var out models.WidgetSetting
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID
		current, err := q.FindWidgetSettings(ctx, models.FindWidgetSettingsParams{WorkspaceID: ws, ID: in.WidgetID})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find widget: %w", err)
		}

		rules, err := readRules(mergeRules(current.Audiences, in.Rules))
		if err != nil {
			return err
		}

		if err := checkRules(ctx, q, ws, rules); err != nil {
			return err
		}

		stored, _ := json.Marshal(rules)
		out, err = q.SaveWidgetAudiences(ctx, models.SaveWidgetAudiencesParams{
			Audiences: stored, Now: s.Clock.Now(), WorkspaceID: ws, ID: in.WidgetID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		return wrapErr("save messenger rules", err)
	})
	if err == nil && s.Bust != nil {
		s.Bust.Forget(ctx, in.WidgetID)
	}

	return out, err
}

/*
readRules reads the rules strictly, each one as an audience.
*/
func readRules(raw json.RawMessage) (audiences.MessengerRules, error) {
	var sent struct {
		Launcher json.RawMessage            `json:"launcher"`
		Start    json.RawMessage            `json:"start"`
		Tabs     map[string]json.RawMessage `json:"tabs"`
	}

	var out audiences.MessengerRules
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&sent); err != nil {
		return out, ErrRulesInvalid.With("reason", err.Error())
	}

	read := func(path string, r json.RawMessage) (*audience.Audience, error) {
		if len(r) == 0 || string(r) == "null" {
			return nil, nil
		}

		a, err := audience.Parse(r)
		if err != nil {
			return nil, ErrRulesInvalid.With("reason", path+": "+err.Error())
		}

		return &a, nil
	}

	var err error
	if out.Launcher, err = read("launcher", sent.Launcher); err != nil {
		return out, err
	}

	if out.Start, err = read("start", sent.Start); err != nil {
		return out, err
	}

	out.Tabs, err = readTabs(sent.Tabs, read)

	return out, err
}

/*
readTabs reads the rule of each tab, leaving out tabs without one.
*/
func readTabs(sent map[string]json.RawMessage, read func(string, json.RawMessage) (*audience.Audience, error),
) (map[string]*audience.Audience, error) {
	var out map[string]*audience.Audience
	for tab, r := range sent {
		a, err := read("tabs."+tab, r)
		if err != nil || a == nil {
			if err != nil {
				return nil, err
			}

			continue
		}

		if out == nil {
			out = map[string]*audience.Audience{}
		}

		out[tab] = a
	}

	return out, nil
}

/*
checkRules compiles every rule and checks the saved audiences they name:
each must exist, and those the start rule names must themselves read
only what the server knows and trusts.
*/
func checkRules(ctx context.Context, q *models.Queries, ws ids.UUID, rules audiences.MessengerRules) error {
	_, found := audiences.CompileMessenger(rules)
	resolver, err := audiences.LoadResolver(ctx, q, ws)
	if err != nil {
		return err
	}

	found = append(found, resolver.CheckNamed(rules)...)
	if len(found) > 0 {
		return ErrRulesInvalid.With("errors", found)
	}

	return nil
}

/*
wrapErr names what failed, leaving no error alone.
*/
func wrapErr(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}

/*
mergeRules lays a change over the stored rules, by rule and by tab: a
rule sent replaces the stored one, null removes it, one left out stays.
*/
func mergeRules(stored, change json.RawMessage) json.RawMessage {
	var base, sent map[string]json.RawMessage
	_ = json.Unmarshal(stored, &base)
	if json.Unmarshal(change, &sent) != nil {
		return change
	}

	if base == nil {
		base = map[string]json.RawMessage{}
	}

	for key, value := range sent {
		if key == "tabs" {
			base[key] = mergeTabs(base[key], value)
			continue
		}

		base[key] = value
	}

	out, _ := json.Marshal(base)

	return out
}

/*
mergeTabs lays a change of tab rules over the stored ones.
*/
func mergeTabs(stored, change json.RawMessage) json.RawMessage {
	var base, sent map[string]json.RawMessage
	_ = json.Unmarshal(stored, &base)
	if json.Unmarshal(change, &sent) != nil || sent == nil {
		return change
	}

	if base == nil {
		base = map[string]json.RawMessage{}
	}

	maps.Copy(base, sent)

	out, _ := json.Marshal(base)

	return out
}
