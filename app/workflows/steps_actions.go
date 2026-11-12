/*
|--------------------------------------------------------------------------
| Action Steps
|--------------------------------------------------------------------------
|
| Actions change the conversation the way a teammate would, through the
| same writer, so the timeline shows "Workflow: <name>" did it and every
| listener hears about it. A conversation a member already holds stays
| with them unless the step says otherwise. An id the workspace no
| longer has is skipped and noted in the trace instead of failing the
| run.
|
*/

package workflows

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The outcomes of action steps.
*/
const (
	outcomeDone        = "done"
	outcomeAssigned    = "assigned"
	outcomeKept        = "kept"
	outcomeNoMember    = "no_eligible_member"
	outcomeMissingRef  = "reference_missing"
	outcomeNotOpen     = "not_open"
	outcomeUnchanged   = "unchanged"
	outcomeAlreadyOpen = "already_open"
)

/*
decode reads a public id, false when it is not one of the prefix.
*/
func (st *state) decode(prefix models.Prefix, public string) (ids.UUID, bool) {
	id, err := st.e.IDs.Decode(string(prefix), public)
	return id, err == nil
}

/*
skipped is the outcome of an action that could not apply.
*/
func skipped(reason string) outcome {
	return outcome{status: "skipped", outcome: reason, acted: true}
}

/*
refused turns a change the conversation refuses, such as an assignee
who left, into a skipped step; any other failure fails the job so it
runs again.
*/
func refused(err error, reason string) (outcome, error) {
	if fault.KindOf(err) == fault.Internal {
		return outcome{}, err
	}

	return skipped(reason), nil
}

/*
runAssign assigns the conversation by the step's mode.
*/
func runAssign(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	a := settings[workflowspec.AssignSettings](s)
	if workflowspec.On(a.KeepIfAssigned, true) && st.conv.AssigneeMemberID != nil {
		return outcome{outcome: outcomeKept, acted: true}, nil
	}

	if a.Mode == "member" {
		member, ok := st.decode(models.PrefixMember, a.MemberID)
		if !ok {
			return skipped(outcomeMissingRef), nil
		}

		return st.assign(ctx, conversations.Assignment{Member: &member, Team: st.conv.AssigneeTeamID})
	}

	team, ok := st.decode(models.PrefixTeam, a.TeamID)
	if !ok {
		return skipped(outcomeMissingRef), nil
	}

	if a.Mode == "team" {
		return st.assign(ctx, conversations.Assignment{Member: st.conv.AssigneeMemberID, Team: &team})
	}

	return st.pick(ctx, a, team)
}

/*
assign hands the conversation over through the writer.
*/
func (st *state) assign(ctx context.Context, to conversations.Assignment) (outcome, error) {
	conv, _, err := st.writer().Assign(ctx, st.conv, to)
	if err != nil {
		return refused(err, outcomeMissingRef)
	}

	st.conv = conv

	return outcome{outcome: outcomeAssigned, acted: true}, nil
}

/*
pick picks a team member by round robin or by the lightest load, as
the inbox's own assignment would, and assigns the team alone when
nobody may take it.
*/
func (st *state) pick(ctx context.Context, a workflowspec.AssignSettings, team ids.UUID) (outcome, error) {
	inbox, err := st.q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: st.conv.WorkspaceID, ID: st.conv.InboxID})
	if err != nil {
		return outcome{}, err
	}

	rules := inbox.ReadSettings().Assignment
	rules.Mode, rules.OverflowTeamID = models.AssignmentRoundRobin, nil
	if a.Mode == "team_balanced" {
		rules.Mode = models.AssignmentBalanced
	}

	res, err := assignment.PickAssignee(ctx, st.q, assignment.PickInput{
		WorkspaceID: st.conv.WorkspaceID, InboxID: st.conv.InboxID, TeamID: &team, Settings: rules,
		ChannelType: enums.ChannelType(st.channel(ctx)), Now: st.now(),
	})
	if err != nil {
		return outcome{}, err
	}

	if res.MemberID == nil {
		if a.Fallback == "leave" {
			return outcome{outcome: outcomeNoMember, acted: true}, nil
		}

		out, err := st.assign(ctx, conversations.Assignment{Team: &team})
		out.outcome = outcomeNoMember

		return out, err
	}

	return st.assign(ctx, conversations.Assignment{Member: res.MemberID, Team: &team})
}

/*
channel is the conversation's channel type.
*/
func (st *state) channel(ctx context.Context) string {
	s, err := st.subject(ctx)
	if err != nil {
		return ""
	}

	return s.channel
}

/*
runUnassign clears the member, the team or both.
*/
func runUnassign(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	u := settings[workflowspec.UnassignSettings](s)
	to := conversations.Assignment{Member: st.conv.AssigneeMemberID, Team: st.conv.AssigneeTeamID}

	if u.What == "member" || u.What == "both" {
		to.Member = nil
	}

	if u.What == "team" || u.What == "both" {
		to.Team = nil
	}

	return st.assign(ctx, to)
}

/*
tagIDs decodes the step's tags, leaving out ones that are not ids.
*/
func (st *state) tagIDs(s workflowspec.Step) []ids.UUID {
	var out []ids.UUID
	for _, public := range settings[workflowspec.TagSettings](s).TagIDs {
		if id, ok := st.decode(models.PrefixTag, public); ok {
			out = append(out, id)
		}
	}

	return out
}

/*
runTag puts tags on the conversation.
*/
func runTag(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	added, err := st.writer().AddTags(ctx, st.conv, tags.Refs{IDs: st.tagIDs(s)}, nil)
	if err != nil {
		return refused(err, outcomeMissingRef)
	}

	return changed(added), nil
}

/*
runUntag takes tags off the conversation.
*/
func runUntag(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	removed, err := st.writer().RemoveTags(ctx, st.conv, st.tagIDs(s))
	if err != nil {
		return outcome{}, err
	}

	return changed(removed), nil
}

/*
changed is the outcome of an action that may have found nothing to do.
*/
func changed(did bool) outcome {
	return outcome{outcome: map[bool]string{true: outcomeDone, false: outcomeUnchanged}[did], acted: true}
}

/*
runPriority sets the conversation's priority.
*/
func runPriority(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	level := enums.ConversationPriority(settings[workflowspec.PrioritySettings](s).Value)

	conv, fields, err := st.writer().SetAttributes(ctx, st.conv, conversations.Attributes{Priority: &level})
	if err != nil {
		return outcome{}, err
	}

	st.conv = conv

	return changed(len(fields) > 0), nil
}

/*
runNote leaves an internal note, mentioning members.
*/
func runNote(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	n := settings[workflowspec.NoteSettings](s)

	text, err := st.render(ctx, n.Text)
	if err != nil {
		return outcome{}, err
	}

	var mentioned []ids.UUID
	for _, public := range n.MentionMemberIDs {
		if id, ok := st.decode(models.PrefixMember, public); ok {
			mentioned = append(mentioned, id)
		}
	}

	_, conv, err := st.writer().Append(ctx, st.conv, conversations.NewMessage{
		Type: string(enums.MessageTypeNote), Body: text, Mentioned: mentioned,
	})
	if err != nil {
		return outcome{}, err
	}

	st.conv = conv

	return outcome{outcome: outcomeDone, acted: true}, nil
}

/*
runUpdateContact sets contact fields.
*/
func runUpdateContact(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	var set []string
	for _, change := range settings[workflowspec.UpdateContactSettings](s).Set {
		value, err := st.render(ctx, change.Value)
		if err != nil {
			return outcome{}, err
		}

		if err := st.store(ctx, change.Field, value); err != nil {
			return outcome{}, err
		}

		set = append(set, change.Field)
	}

	return outcome{outcome: outcomeDone, acted: true, detail: map[string]any{"fields": set}}, nil
}

/*
runSnooze snoozes an open conversation.
*/
func runSnooze(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	sn := settings[workflowspec.SnoozeSettings](s)
	if st.conv.Status == string(enums.ConversationStatusClosed) {
		return skipped(outcomeNotOpen), nil
	}

	until, err := st.snoozeEnd(ctx, sn)
	if scheduleProblem(err) {
		return skipped(outcomeMissingRef), nil
	}

	if err != nil || !until.After(st.now()) {
		return skipped(outcomeAlreadyOpen), err
	}

	return st.changeStatus(ctx, conversations.StatusChange{
		Status: string(enums.ConversationStatusSnoozed), Until: until,
	})
}

/*
snoozeEnd is when a snooze ends: after its duration, at the next
opening of its schedule, or at the next time of day.
*/
func (st *state) snoozeEnd(ctx context.Context, sn workflowspec.SnoozeSettings) (time.Time, error) {
	switch {
	case sn.Until != nil && sn.Until.Kind == untilOpen:
		return st.nextOpen(ctx, sn.Until)
	case sn.Until != nil:
		return st.nextTimeOfDay(ctx, sn.Until.Time), nil
	}

	d, err := workflowspec.ParseDuration(sn.For)
	if err != nil {
		d = 24 * time.Hour
	}

	return st.now().Add(d), nil
}

/*
runClose closes the conversation, unless the step leaves conversations
a teammate answered alone, and asks the customer for a rating when the
step says so.
*/
func runClose(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	c := settings[workflowspec.CloseSettings](s)
	if c.OnlyIfNoTeammateReply && st.conv.FirstReplyAt != nil {
		return skipped("teammate_replied"), nil
	}

	return st.changeStatus(ctx, conversations.StatusChange{
		Status: string(enums.ConversationStatusClosed), AskRating: c.AskRating,
	})
}

/*
changeStatus moves the conversation through the writer.
*/
func (st *state) changeStatus(ctx context.Context, to conversations.StatusChange) (outcome, error) {
	conv, did, err := st.writer().ChangeStatus(ctx, st.conv, to)
	if err != nil {
		return refused(err, outcomeNotOpen)
	}

	st.conv = conv

	return changed(did), nil
}
