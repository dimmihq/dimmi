/*
|--------------------------------------------------------------------------
| People And Tags
|--------------------------------------------------------------------------
|
| The workspace's tags and the people a conversation can go to, with the
| ids Operator needs to propose tagging and assigning. The small helpers
| that write rows as text for the model live here too.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listTags lists the workspace's tags.
*/
func (e *Engine) listTags(ctx context.Context, s *session, _ json.RawMessage) (result, error) {
	if err := conversations.CanRead(s.m); err != nil {
		return result{}, err
	}
	var tags []models.Tag
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		tags, err = models.New(tx).ListTags(ctx, models.ListTagsParams{WorkspaceID: s.m.Workspace.ID})
		return err
	})
	if err != nil {
		return result{}, fmt.Errorf("list tags: %w", err)
	}
	text := tagNames(tags)
	return result{text: text, step: Step{Tool: "list_tags", Summary: fmt.Sprintf("Listed %d tags", len(tags))}}, nil
}

/*
listTeammates lists the active teammates and teams with their ids.
*/
func (e *Engine) listTeammates(ctx context.Context, s *session, _ json.RawMessage) (result, error) {
	if err := conversations.CanRead(s.m); err != nil {
		return result{}, err
	}
	var b strings.Builder
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		members, err := q.ListWorkspaceMembers(ctx, models.ListWorkspaceMembersParams{
			WorkspaceID: s.m.Workspace.ID, Statuses: []string{"active"}, RowLimit: 500,
		})
		if err != nil {
			return err
		}
		b.WriteString("Teammates:\n")
		for _, m := range members {
			fmt.Fprintf(&b, "- %s <%s>, %s, %s (id %s)\n", m.User.Name, m.User.Email, m.WorkspaceMember.Role,
				m.WorkspaceMember.Availability, models.PublicID(models.PrefixMember, m.WorkspaceMember.ID))
		}
		teams, err := q.ListTeams(ctx, models.ListTeamsParams{WorkspaceID: s.m.Workspace.ID, RowLimit: 200})
		if err != nil {
			return err
		}
		b.WriteString("Teams:\n")
		for _, t := range teams {
			fmt.Fprintf(&b, "- %s, %d members (id %s)\n", t.Team.Name, t.MemberCount,
				models.PublicID(models.PrefixTeam, t.Team.ID))
		}
		return nil
	})
	if err != nil {
		return result{}, fmt.Errorf("list teammates: %w", err)
	}
	return result{text: b.String(), step: Step{Tool: "list_teammates", Summary: "Listed teammates and teams"}}, nil
}

/*
untrusted wraps text an outsider wrote, so the model reads it as data.
*/
func untrusted(text string) string {
	if text == "" {
		return ""
	}
	text = strings.NewReplacer("<untrusted>", "", "</untrusted>", "").Replace(text)
	return "<untrusted>" + text + "</untrusted>"
}

/*
deref reads an optional string.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
dateOf writes an optional time as a date.
*/
func dateOf(t *time.Time) string {
	if t == nil {
		return "unknown"
	}
	return t.UTC().Format("2006-01-02")
}

/*
assigneeOf names a conversation's assignee.
*/
func assigneeOf(l conversations.Loaded) string {
	if l.Assignee == nil {
		return "nobody"
	}
	return fmt.Sprintf("%s (id %s)", l.Assignee.User.Name, models.PublicID(models.PrefixMember, l.Assignee.Member.ID))
}

/*
teamOf names a conversation's team.
*/
func teamOf(l conversations.Loaded) string {
	if l.Team == nil {
		return "none"
	}
	return fmt.Sprintf("%s (id %s)", l.Team.Name, models.PublicID(models.PrefixTeam, l.Team.ID))
}

/*
tagNames lists tags by name.
*/
func tagNames(tags []models.Tag) string {
	if len(tags) == 0 {
		return "none"
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
