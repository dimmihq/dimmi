/*
|--------------------------------------------------------------------------
| Triage Tool
|--------------------------------------------------------------------------
|
| The agent keeps the conversation triaged as it learns: a title that
| covers every topic so far, the tags that describe it now, the team it
| belongs to, and whether it is urgent. triage_conversation is
| declarative and idempotent: it says what should hold, a field left out
| stays as it is, and calling it again with the same values changes
| nothing.
|
| A teammate's decision always wins. A subject a person set stays, tags
| a person added are never removed, an assignment a person made is never
| overridden, and a priority a person set is never cleared; the result
| tells the model what it kept. Only existing tags and teams count; an
| unknown name is ignored and reported. The tool records what it wants;
| the commit applies it through the conversation actions with the agent
| as actor, so the timeline, the audit and the events see it.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
toolTriage is the triage tool's name.
*/
const toolTriage = "triage_conversation"

/*
Triage limits: names listed in the description, and the title's
length in runes.
*/
const (
	namesListed = 60
	titleRunes  = 120
)

/*
changeSet is the triage the tool asked for, applied at commit.
*/
type changeSet struct {
	title      *string
	addTags    []ids.UUID
	removeTags []ids.UUID
	team       *ids.UUID
	priority   *bool
}

/*
triageFacts are what decides whether the agent may change something: who
put each tag on, and whether a person set the subject, the assignment
or the priority.
*/
type triageFacts struct {
	tagAuthors     map[ids.UUID]string
	humanSubject   bool
	humanAssignee  bool
	humanPriority  bool
	workspaceTags  []models.Tag
	workspaceTeams []models.Team
}

/*
triageSpec describes the tool with the workspace's tags and teams.
*/
func triageSpec(e *env) contracts.ToolSpec {
	tagNames := make([]string, 0, len(e.Triage.workspaceTags))
	for _, t := range e.Triage.workspaceTags {
		tagNames = append(tagNames, t.Name)
	}
	teamNames := make([]string, 0, len(e.Triage.workspaceTeams))
	for _, t := range e.Triage.workspaceTeams {
		teamNames = append(teamNames, t.Name)
	}
	return contracts.ToolSpec{
		Name: toolTriage,
		Description: "Keep the conversation triaged for the inbox; call it again as you learn more. Every field is " +
			"optional and a field left out stays unchanged. title: 3 to 8 words in the workspace's language " +
			"covering every topic raised so far. tags: the complete set of tags that describe the conversation " +
			"now; only these exist: " + listed(tagNames) + ". team: only these exist: " + listed(teamNames) +
			". priority: true for urgent business impact, an outage, a security or legal matter, or a very " +
			"frustrated customer. A teammate's choices are kept; the result says what was applied, ignored or kept.",
		Schema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},` +
			`"tags":{"type":"array","items":{"type":"string"}},"team":{"type":"string"},` +
			`"priority":{"type":"boolean"}},"additionalProperties":false}`),
	}
}

/*
triageInput is the tool's arguments.
*/
type triageInput struct {
	Title    *string   `json:"title"`
	Tags     *[]string `json:"tags"`
	Team     *string   `json:"team"`
	Priority *bool     `json:"priority"`
}

/*
triageResult is what the model reads back.
*/
type triageResult struct {
	Applied map[string]any `json:"applied"`
	Ignored []string       `json:"ignored"`
	Kept    []string       `json:"kept_human_changes"`
}

/*
triage records the desired triage and reports what will hold.
*/
func (r *Runner) triage(_ context.Context, st *run, args json.RawMessage) (string, error) {
	var in triageInput
	if err := json.Unmarshal(args, &in); err != nil {
		return "", errToolArguments
	}
	res := triageResult{Applied: map[string]any{}, Ignored: []string{}, Kept: []string{}}
	f := st.env.Triage

	st.triageTitle(in.Title, &res)
	if in.Tags != nil {
		st.triageTags(*in.Tags, &res)
	}
	if in.Team != nil {
		st.triageTeam(*in.Team, &res)
	}
	if in.Priority != nil {
		if !*in.Priority && f.humanPriority {
			res.Kept = append(res.Kept, "priority set by a teammate")
		} else {
			st.changes.priority = in.Priority
			res.Applied["priority"] = *in.Priority
		}
	}

	out, err := json.Marshal(res)
	return string(out), err
}

/*
triageTitle records a new title unless a person set the subject.
*/
func (st *run) triageTitle(title *string, res *triageResult) {
	if title == nil || strings.TrimSpace(*title) == "" {
		return
	}
	if st.env.Triage.humanSubject {
		res.Kept = append(res.Kept, "title set by a teammate or the customer")
		return
	}
	clean := truncate(strings.Trim(strings.TrimSpace(*title), `"'`), titleRunes)
	st.changes.title = &clean
	res.Applied["title"] = clean
}

/*
triageTags turns the desired set into tags to add and agent tags to
remove; a person's tags stay whatever the set says.
*/
func (st *run) triageTags(names []string, res *triageResult) {
	f := st.env.Triage
	want := map[ids.UUID]bool{}
	var applied []string
	for _, name := range names {
		i := slices.IndexFunc(f.workspaceTags, func(t models.Tag) bool { return strings.EqualFold(t.Name, name) })
		if i < 0 {
			res.Ignored = append(res.Ignored, "tag "+name)
			continue
		}
		want[f.workspaceTags[i].ID] = true
		applied = append(applied, f.workspaceTags[i].Name)
	}

	st.changes.addTags, st.changes.removeTags = nil, nil
	for id := range want {
		if _, on := f.tagAuthors[id]; !on {
			st.changes.addTags = append(st.changes.addTags, id)
		}
	}
	for id, author := range f.tagAuthors {
		switch {
		case want[id]:
		case author == string(enums.AuthorTypeAgent):
			st.changes.removeTags = append(st.changes.removeTags, id)
		default:
			res.Kept = append(res.Kept, "tag "+tagName(f, id)+" added by a teammate")
		}
	}
	res.Applied["tags"] = applied
}

/*
triageTeam records the team unless a person assigned the conversation.
*/
func (st *run) triageTeam(name string, res *triageResult) {
	f := st.env.Triage
	i := slices.IndexFunc(f.workspaceTeams, func(t models.Team) bool { return strings.EqualFold(t.Name, name) })
	if i < 0 {
		res.Ignored = append(res.Ignored, "team "+name)
		return
	}
	if st.env.Conversation.AssigneeMemberID != nil || f.humanAssignee {
		res.Kept = append(res.Kept, "assignment made by a teammate")
		return
	}
	st.changes.team = &f.workspaceTeams[i].ID
	res.Applied["team"] = f.workspaceTeams[i].Name
}

/*
tagName names a tag id from the workspace's list.
*/
func tagName(f triageFacts, id ids.UUID) string {
	for _, t := range f.workspaceTags {
		if t.ID == id {
			return t.Name
		}
	}
	return id.String()
}

/*
listed renders names for a description.
*/
func listed(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	if len(names) > namesListed {
		names = names[:namesListed]
	}
	return strings.Join(names, ", ")
}

/*
loadTriage reads the workspace's tags and teams and who made the
conversation's current triage.
*/
func loadTriage(ctx context.Context, q *models.Queries, e *env) (triageFacts, error) {
	ws, conv := e.Conversation.WorkspaceID, e.Conversation.ID
	f := triageFacts{tagAuthors: map[ids.UUID]string{}}
	var err error
	f.workspaceTags, err = q.ListAgentTags(ctx, models.ListAgentTagsParams{WorkspaceID: ws, RowLimit: 200})
	if err != nil {
		return f, fmt.Errorf("list tags: %w", err)
	}
	if f.workspaceTeams, err = q.ListAgentTeams(ctx, models.ListAgentTeamsParams{
		WorkspaceID: ws, RowLimit: 200,
	}); err != nil {
		return f, fmt.Errorf("list teams: %w", err)
	}
	authors, err := q.ListConversationTagAuthors(ctx, models.ListConversationTagAuthorsParams{
		WorkspaceID: ws, ConversationID: conv,
	})
	if err != nil {
		return f, fmt.Errorf("list conversation tags: %w", err)
	}
	for _, a := range authors {
		f.tagAuthors[a.TagID] = a.CreatedByType
	}

	subject := lastAuthor(ctx, q, e, change{"conversation.updated", "subject"})
	f.humanSubject = strv(e.Conversation.Subject) != "" && subject != string(enums.AuthorTypeAgent)
	assignee := lastAuthor(ctx, q, e, change{name: "conversation.assigned"})
	f.humanAssignee = e.Conversation.AssigneeTeamID != nil && assignee == string(enums.AuthorTypeMember)
	f.humanPriority = lastAuthor(ctx, q, e, change{name: "conversation.priority_changed"}) ==
		string(enums.AuthorTypeMember)
	return f, nil
}

/*
change is a kind of timeline change: its event name and, optionally,
the field it changed.
*/
type change struct {
	name, field string
}

/*
lastAuthor names who made the newest change of the kind, "" when none
is on the timeline.
*/
func lastAuthor(ctx context.Context, q *models.Queries, e *env, c change) string {
	author, err := q.LastChangeAuthor(ctx, models.LastChangeAuthorParams{
		WorkspaceID: e.Conversation.WorkspaceID, ConversationID: e.Conversation.ID, Name: c.name, Field: c.field,
	})
	if err != nil {
		return ""
	}
	return author
}

/*
applyChanges applies the recorded triage as the agent.
*/
func (r *Runner) applyChanges(ctx context.Context, tx *database.Tx, st *run) error {
	e, ch := st.env, st.changes
	w := r.agentWriter(tx, e)

	if _, err := w.AddTags(ctx, e.Conversation, tags.Refs{IDs: ch.addTags}, nil); err != nil {
		return fmt.Errorf("add tags: %w", err)
	}
	if _, err := w.RemoveTags(ctx, e.Conversation, ch.removeTags); err != nil {
		return fmt.Errorf("remove tags: %w", err)
	}

	attrs := conversations.Attributes{Subject: ch.title}
	if ch.priority != nil {
		level := enums.ConversationPriorityNone
		if *ch.priority {
			level = enums.ConversationPriorityHigh
		}
		attrs.Priority = &level
	}
	if attrs.Subject != nil || attrs.Priority != nil {
		updated, _, err := w.SetAttributes(ctx, e.Conversation, attrs)
		if err != nil {
			return fmt.Errorf("triage conversation: %w", err)
		}
		e.Conversation = updated
	}

	if ch.team == nil || e.Conversation.AssigneeMemberID != nil {
		return nil
	}
	assigned, _, err := w.Assign(ctx, e.Conversation, conversations.Assignment{Team: ch.team})
	if err != nil {
		return fmt.Errorf("assign team: %w", err)
	}
	e.Conversation = assigned
	return nil
}
