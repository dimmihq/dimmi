/*
|--------------------------------------------------------------------------
| Conversation State And Sync
|--------------------------------------------------------------------------
|
| The state a conversation ended in is applied once its entries are in.
| A later sync compares each field with what the migration last wrote,
| and leaves alone, and reports, one a teammate changed here since.
|
*/

package migrations

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
fields returns the conversation's fields as a sync compares them.
*/
func (r *replay) fields(ctx context.Context) map[string]string {
	out := map[string]string{
		fieldStatus: r.conv.Status, fieldPriority: priorityName(r.conv.Priority),
		fieldAssignee: idText(r.conv.AssigneeMemberID), fieldTeam: idText(r.conv.AssigneeTeamID),
	}

	rows, err := r.s.q.ListConversationTags(ctx, models.ListConversationTagsParams{
		WorkspaceID: r.s.ws(), ConversationIds: []ids.UUID{r.conv.ID},
	})
	if err == nil {
		var list []string
		for _, row := range rows {
			list = append(list, row.Tag.ID.String())
		}

		sort.Strings(list)
		out[fieldTags] = strings.Join(list, ",")
	}

	return out
}

/*
priorityName returns the name of a priority level.
*/
func priorityName(level int16) string {
	for _, p := range []enums.ConversationPriority{
		enums.ConversationPriorityNone, enums.ConversationPriorityLow, enums.ConversationPriorityMedium,
		enums.ConversationPriorityHigh, enums.ConversationPriorityUrgent,
	} {
		if l, ok := p.Level(); ok && l == level {
			return string(p)
		}
	}

	return ""
}

/*
idText returns the id as text, empty for nil.
*/
func idText(id *ids.UUID) string {
	if id == nil {
		return ""
	}

	return id.String()
}

/*
findConflicts notes each field a teammate changed here since the
migration last wrote it.
*/
func (r *replay) findConflicts(ctx context.Context) {
	r.conflicts = map[string]bool{}

	for field, now := range r.fields(ctx) {
		last, ok := r.state.Fields[field]
		if ok && last != now {
			r.conflicts[field] = true
		}
	}
}

/*
final moves the conversation to the state it ended in: its status,
priority, assignee, team and tags, each when sent and not in conflict,
and remembers what the migration left.
*/
func (r *replay) final(ctx context.Context) error {
	w := r.w.Importing(conversations.Latest(r.conv)).AsSystem()
	now := r.fields(ctx)
	want := r.wanted()

	for _, field := range slices.Sorted(maps.Keys(want)) {
		value := want[field]
		if value == now[field] {
			continue
		}

		if r.conflicts[field] {
			r.conflict(field, now[field], value)

			continue
		}

		if err := r.set(ctx, w, field); err != nil {
			return err
		}
	}

	if err := r.finalTags(ctx, w, now); err != nil {
		return err
	}

	for field, value := range r.fields(ctx) {
		if !r.conflicts[field] {
			r.state.Fields[field] = value
		}
	}

	return nil
}

/*
wanted returns the value of each field the record sent a final state
for, as a sync compares them.
*/
func (r *replay) wanted() map[string]string {
	c := r.c
	want := map[string]string{}

	if c.Status != "" {
		want[fieldStatus] = c.Status
	}

	if c.Priority != "" {
		want[fieldPriority] = c.Priority
	}

	if c.AssigneeRef != "" {
		id, _ := r.s.refs.id(KindTeammate, c.AssigneeRef)
		want[fieldAssignee] = idText(nilIfZero(id))
	}

	if c.TeamRef != "" {
		id, _ := r.s.refs.id(KindTeam, c.TeamRef)
		want[fieldTeam] = idText(nilIfZero(id))
	}

	return want
}

/*
finalTags puts the record's final tags on the conversation, unless a
teammate changed its tags here since.
*/
func (r *replay) finalTags(ctx context.Context, w *conversations.Writer, now map[string]string) error {
	if len(r.c.TagRefs) == 0 {
		return nil
	}

	if r.conflicts[fieldTags] {
		want := strings.Join(slices.Sorted(slices.Values(idTexts(r.s.refs.ids(KindTag, r.c.TagRefs)))), ",")
		r.conflict(fieldTags, now[fieldTags], want)

		return nil
	}

	return r.tags(ctx, w, r.c.TagRefs, nil)
}

/*
conflict reports a field a teammate changed here, when the other tool
changed it too since the migration last wrote it. Values are public ids
for teammates, teams and tags.
*/
func (r *replay) conflict(field, here, source string) {
	if source == r.state.Fields[field] {
		return
	}

	r.out.conflicts = append(r.out.conflicts, Conflict{
		Field: field, Dimmi: publicValue(field, here), Source: publicValue(field, source),
	})
}

/*
publicValue returns a compared value as a client reads it: public ids
for a teammate, team or tags.
*/
func publicValue(field, value string) any {
	prefix, ok := map[string]models.Prefix{
		fieldAssignee: models.PrefixMember, fieldTeam: models.PrefixTeam, fieldTags: models.PrefixTag,
	}[field]
	if !ok || value == "" {
		return value
	}

	var out []string

	for raw := range strings.SplitSeq(value, ",") {
		if id, err := ids.Parse(raw); err == nil {
			out = append(out, models.PublicID(prefix, id))
		}
	}

	if field == fieldTags {
		return out
	}

	return strings.Join(out, ",")
}

/*
idTexts returns the ids as text.
*/
func idTexts(list []ids.UUID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, id.String())
	}

	return out
}

/*
nilIfZero returns nil for the nil id.
*/
func nilIfZero(id ids.UUID) *ids.UUID {
	if id == ids.Nil() {
		return nil
	}

	return &id
}

/*
set applies one field of the final state.
*/
func (r *replay) set(ctx context.Context, w *conversations.Writer, field string) error {
	c := r.c

	switch field {
	case fieldStatus:
		return r.status(ctx, w, c.Status, c.SnoozedUntil)
	case fieldPriority:
		return r.priority(ctx, w, c.Priority)
	case fieldAssignee:
		ref := c.AssigneeRef
		return r.assign(ctx, w, target{kind: KindTeammate, ref: &ref})
	case fieldTeam:
		ref := c.TeamRef
		return r.assign(ctx, w, target{kind: KindTeam, ref: &ref})
	}

	return nil
}

/*
rating keeps how the customer rated the conversation, once it is
closed.
*/
func (r *replay) rating(ctx context.Context) error {
	rt := r.c.Rating
	if rt == nil {
		return nil
	}

	if r.conv.Status != string(enums.ConversationStatusClosed) || r.conv.ClosedAt == nil {
		r.out.warn("rating", "conversation_not_closed", "A rating is kept only on a closed conversation.")

		return nil
	}

	channel := "widget"
	if r.c.Channel == "email" {
		channel = "email"
	}

	member := r.conv.AssigneeMemberID
	if id, ok := r.s.refs.id(KindTeammate, rt.TeammateRef); ok {
		member = &id
	}

	err := r.s.q.MigrateRating(ctx, models.MigrateRatingParams{
		WorkspaceID: r.s.ws(), ConversationID: r.conv.ID, ContactID: r.conv.ContactID,
		MemberID: member, Channel: channel, ClosedAt: *r.conv.ClosedAt,
		RatedAt: rt.CreatedAt.UTC(), Score: new(int16(rt.Score)), //nolint:gosec // 1 to 5
		Comment: orNil(cut(rt.Remark, 2000)),
	})
	if err != nil {
		return fmt.Errorf("keep rating: %w", err)
	}

	return nil
}
