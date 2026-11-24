/*
|--------------------------------------------------------------------------
| Dry Run Actions
|--------------------------------------------------------------------------
|
| An action in a dry run only says what it would do. Where a later step
| could read the change, such as a tag a branch checks or the team a
| condition names, the dry run's own copy of the conversation changes
| too; the real conversation never does.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
dryAction is how one kind of action goes in a dry run.
*/
type dryAction func(d *dry, ctx context.Context, s workflowspec.Step) outcome

/*
dryActions are the actions a dry run knows by kind.
*/
func dryActions() map[string]dryAction {
	return map[string]dryAction{
		workflowspec.KindTag: (*dry).tag, workflowspec.KindUntag: (*dry).untag,
		workflowspec.KindPriority: (*dry).priority, workflowspec.KindUnassign: (*dry).unassign,
		workflowspec.KindNote: (*dry).note, workflowspec.KindUpdateContact: (*dry).updateContact,
		workflowspec.KindAssign: func(d *dry, _ context.Context, s workflowspec.Step) outcome { return d.assign(s) },
		workflowspec.KindSnooze: func(d *dry, _ context.Context, s workflowspec.Step) outcome {
			d.conv.Status = "snoozed"
			return d.act(s, "Would snooze the conversation", nil)
		},
		workflowspec.KindClose: func(d *dry, _ context.Context, s workflowspec.Step) outcome {
			d.conv.Status = "closed"
			return d.act(s, "Would close the conversation", nil)
		},
		workflowspec.KindApplySLA: func(d *dry, _ context.Context, s workflowspec.Step) outcome {
			id := settings[workflowspec.ApplySLASettings](s).SLAPolicyID
			return d.act(s, "Would apply an SLA policy", map[string]any{"sla_policy_id": id})
		},
	}
}

/*
action records what an action would do. A kind this file does not know
is recorded as what would run when it is an action, and as not
simulated otherwise.
*/
func (d *dry) action(ctx context.Context, s workflowspec.Step) outcome {
	if run, ok := dryActions()[s.Kind]; ok {
		return run(d, ctx, s)
	}
	if k, ok := workflowspec.FindKind(s.Kind); ok && k.Action {
		var detail map[string]any
		_ = json.Unmarshal(s.Settings, &detail)
		return d.act(s, "Would run "+strings.ReplaceAll(s.Kind, "_", " "), detail)
	}

	return outcome{outcome: "not_simulated", status: "skipped"}
}

/*
tag adds tags to the dry run's copy.
*/
func (d *dry) tag(_ context.Context, s workflowspec.Step) outcome {
	tags := settings[workflowspec.TagSettings](s).TagIDs
	for _, t := range tags {
		if !slices.Contains(d.subj.conversationTags, t) {
			d.subj.conversationTags = append(d.subj.conversationTags, t)
		}
	}
	return d.act(s, "Would add tags", map[string]any{"tag_ids": tags})
}

/*
untag takes tags off the dry run's copy.
*/
func (d *dry) untag(_ context.Context, s workflowspec.Step) outcome {
	tags := settings[workflowspec.TagSettings](s).TagIDs
	d.subj.conversationTags = slices.DeleteFunc(d.subj.conversationTags, func(t string) bool {
		return slices.Contains(tags, t)
	})
	return d.act(s, "Would remove tags", map[string]any{"tag_ids": tags})
}

/*
priority sets the priority of the dry run's copy.
*/
func (d *dry) priority(_ context.Context, s workflowspec.Step) outcome {
	value := settings[workflowspec.PrioritySettings](s).Value
	if level, ok := enums.ConversationPriority(value).Level(); ok {
		d.conv.Priority = level
	}
	return d.act(s, "Would set the priority to "+value, map[string]any{"priority": value})
}

/*
unassign clears the assignee or team of the dry run's copy.
*/
func (d *dry) unassign(_ context.Context, s workflowspec.Step) outcome {
	what := settings[workflowspec.UnassignSettings](s).What
	if what != "team" {
		d.conv.AssigneeMemberID = nil
	}
	if what != "member" {
		d.conv.AssigneeTeamID = nil
	}
	return d.act(s, "Would unassign the conversation", map[string]any{"what": what})
}

/*
note records the note the workflow would leave.
*/
func (d *dry) note(ctx context.Context, s workflowspec.Step) outcome {
	text := d.render(ctx, settings[workflowspec.NoteSettings](s).Text)
	return d.act(s, "Would leave a note", map[string]any{"text": text})
}

/*
updateContact changes the contact of the dry run's copy.
*/
func (d *dry) updateContact(ctx context.Context, s workflowspec.Step) outcome {
	var set []map[string]any
	for _, c := range settings[workflowspec.UpdateContactSettings](s).Set {
		value := d.render(ctx, c.Value)
		d.store(c.Field, value)
		set = append(set, map[string]any{"field": c.Field, "value": value})
	}
	return d.act(s, "Would update the contact", map[string]any{"set": set})
}

/*
assign records an assignment and makes it in the dry run's copy.
*/
func (d *dry) assign(s workflowspec.Step) outcome {
	a := settings[workflowspec.AssignSettings](s)
	if workflowspec.On(a.KeepIfAssigned, true) && d.conv.AssigneeMemberID != nil {
		return outcome{outcome: outcomeKept, acted: true}
	}
	detail := map[string]any{"mode": a.Mode}
	if a.Mode == "member" {
		if id, ok := d.decode(models.PrefixMember, a.MemberID); ok {
			d.conv.AssigneeMemberID = &id
		}
		detail["member_id"] = a.MemberID
		return d.act(s, "Would assign to a teammate", detail)
	}
	if id, ok := d.decode(models.PrefixTeam, a.TeamID); ok {
		d.conv.AssigneeTeamID = &id
	}
	detail["team_id"] = a.TeamID
	summary := "Would assign to a team"
	if a.Mode != "team" {
		summary = fmt.Sprintf("Would assign to a teammate of the team (%s)", strings.ReplaceAll(a.Mode, "_", " "))
	}

	return d.act(s, summary, detail)
}
