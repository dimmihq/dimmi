/*
|--------------------------------------------------------------------------
| Saved Replies And Schedules
|--------------------------------------------------------------------------
|
| Saved replies keep what sending them does, and schedules their office
| hours and days off, each matched to one the workspace already has.
|
*/

package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
Saved replies keep their actions: a tag, an assignee or team, a status.
*/
func (r *SavedReply) validate() error {
	if err := required("title", r.Title); err != nil {
		return err
	}

	if err := required("body", r.Body); err != nil {
		return err
	}

	for i, a := range r.Actions {
		if err := oneOf(fmt.Sprintf("actions[%d].type", i), a.Type, "tag", "assign", "status", "priority"); err != nil {
			return err
		}
	}

	return nil
}

func (r *SavedReply) refs() []Ref {
	out := refsOf(KindTeammate, r.OwnerRef)
	for _, a := range r.Actions {
		out = append(out, refsOf(KindTag, a.TagRef)...)
		out = append(out, refsOf(KindTeammate, a.AssigneeRef)...)
		out = append(out, refsOf(KindTeam, a.TeamRef)...)
	}

	return out
}

func (r *SavedReply) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: r.Title}

	shared := r.shared()

	fields := savedreplies.Fields{
		Title: new(cut(r.Title, 120)), Body: &r.Body, Shortcut: orNil(r.Shortcut), Shared: &shared,
		Macro: r.macro(s, &out),
	}

	m, err := s.replyOwner(ctx, r)
	if err != nil {
		return out, err
	}

	if s.prev != nil && s.prev.RecordID != nil {
		updated, err := s.kit().UpdateSavedReply.Handle(ctx, savedreplies.UpdateSavedReplyInput{
			Membership: m, ID: *s.prev.RecordID, Fields: fields,
		})
		if err == nil {
			out.id = updated.ID

			return out, nil
		}
	}

	made, err := s.kit().CreateSavedReply.Handle(ctx, savedreplies.CreateSavedReplyInput{Membership: m, Fields: fields})
	if err != nil {
		return out, err
	}

	out.id, out.created = made.ID, true

	return out, nil
}

/*
shared reports whether the reply is the whole team's: as sent, else
when it names no owner.
*/
func (r *SavedReply) shared() bool {
	if r.Shared != nil {
		return *r.Shared
	}

	return r.OwnerRef == ""
}

/*
macro returns what sending the reply does, leaving out what names a
record that is not here.
*/
func (r *SavedReply) macro(s *step, out *outcome) *savedreplies.Macro {
	if len(r.Actions) == 0 {
		return nil
	}

	m := &savedreplies.Macro{}

	for _, a := range r.Actions {
		switch a.Type {
		case "tag":
			if id, ok := s.refs.id(KindTag, a.TagRef); ok {
				m.TagIDs = append(m.TagIDs, id)
			}
		case "assign":
			if id, ok := s.refs.id(KindTeammate, a.AssigneeRef); ok {
				m.AssigneeID = &id
			} else if a.AssigneeRef != "" {
				out.warn("actions", "teammate_unavailable", "An assignment to a teammate not here yet was left out.")
			}

			if id, ok := s.refs.id(KindTeam, a.TeamRef); ok {
				m.TeamID = &id
			}
		case "status":
			m.Status = orNil(a.Status)
		case "priority":
			m.Priority = orNil(a.Status)
		}
	}

	return m
}

/*
replyOwner returns the membership a personal reply is written as: its
owner when they are a member here, else the caller's.
*/
func (s *step) replyOwner(ctx context.Context, r *SavedReply) (authz.Membership, error) {
	m := s.runner.m
	if r.shared() {
		return m, nil
	}

	id, ok := s.refs.id(KindTeammate, r.OwnerRef)
	if !ok {
		return m, nil
	}

	member, err := s.q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: s.ws(), ID: id})
	if err != nil {
		return m, fmt.Errorf("find reply owner: %w", err)
	}

	m.Member = member

	return m, nil
}

/*
Schedules.
*/
func (sc *Schedule) validate() error {
	if err := required("name", sc.Name); err != nil {
		return err
	}

	if err := required("timezone", sc.Timezone); err != nil {
		return err
	}

	for i, h := range sc.Weekly {
		days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
		if err := oneOf(fmt.Sprintf("weekly[%d].day", i), h.Day, days...); err != nil {
			return err
		}

		if err := required(fmt.Sprintf("weekly[%d].day", i), h.Day); err != nil {
			return err
		}
	}

	return nil
}

func (sc *Schedule) refs() []Ref { return nil }

func (sc *Schedule) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: sc.Name}

	weekly := officehours.Weekly{}
	for _, h := range sc.Weekly {
		weekly[h.Day] = append(weekly[h.Day], [2]string{h.Start, h.End})
	}

	holidays := make([]officehours.Holiday, 0, len(sc.Holidays))
	for _, h := range sc.Holidays {
		holidays = append(holidays, officehours.Holiday{Date: h.Date, Name: h.Name, Yearly: h.Yearly})
	}

	for _, e := range sc.Exceptions {
		if !e.Closed {
			out.unsupported("exceptions")

			continue
		}

		holidays = append(holidays, officehours.Holiday{Date: e.Date, Name: e.Name, Yearly: e.Yearly})
	}

	name := cut(strings.TrimSpace(sc.Name), 60)
	in := schedules.Input{
		Membership: s.runner.m, Name: &name, Timezone: &sc.Timezone, Weekly: weekly, Holidays: holidays,
		Default: sc.Default,
	}

	if id, ok := s.existingSchedule(ctx, name); ok {
		in.ID = id

		updated, err := s.kit().Schedules.Update(ctx, in)
		out.id = updated.ID

		return out, err
	}

	made, err := s.kit().Schedules.Create(ctx, in)
	if err != nil {
		return out, err
	}

	out.id, out.created = made.ID, true

	return out, nil
}

/*
existingSchedule returns the schedule this record made before, else the
workspace's schedule with the name.
*/
func (s *step) existingSchedule(ctx context.Context, name string) (ids.UUID, bool) {
	list, err := s.kit().Schedules.List(ctx, s.runner.m)
	if err != nil {
		return ids.Nil(), false
	}

	for _, sc := range list {
		if s.prev != nil && s.prev.RecordID != nil && sc.ID == *s.prev.RecordID {
			return sc.ID, true
		}
	}

	for _, sc := range list {
		if strings.EqualFold(sc.Name, name) {
			return sc.ID, true
		}
	}

	return ids.Nil(), false
}
