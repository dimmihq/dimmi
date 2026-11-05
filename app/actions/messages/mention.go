/*
|--------------------------------------------------------------------------
| Note Mentions
|--------------------------------------------------------------------------
|
| A note mentions teammates with <@mem_...> and teams with <@team_...>.
| Each mentioned teammate gets a row, a team giving one row to each of its
| active members, so the Mentions view and notifications read rows and
| never the note's text. The writer never mentions themself, a teammate
| who left is not mentioned, and with team mentions off a team token
| stays plain text. A teammate who cannot see the conversation still gets
| a row; the Mentions view only lists what they may see, so the mention
| shows once their access grows. Editing a note keeps the rows in step
| with its text and reports only the teammates it newly mentions.
|
*/

package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/mentions"
)

/*
mentioned is who a note mentions: each member with the team that brought
them in, nil when named directly, and the tokens as written with their
names, kept on the note for showing it.
*/
type mentioned struct {
	members map[ids.UUID]*ids.UUID
	tokens  []mentionToken
}

/*
mentionToken is a mention as the note shows it.
*/
type mentionToken struct {
	Kind string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
metadata returns the note's mentions as stored in its metadata.
*/
func (m mentioned) metadata() (json.RawMessage, error) {
	raw, err := json.Marshal(map[string]any{"mentions": m.tokens})
	if err != nil {
		return nil, fmt.Errorf("encode mentions: %w", err)
	}

	return raw, nil
}

/*
ids returns the mentioned members, in a stable order.
*/
func (m mentioned) ids() []ids.UUID {
	out := make([]ids.UUID, 0, len(m.members))
	for id := range m.members {
		out = append(out, id)
	}

	slices.SortFunc(out, func(a, b ids.UUID) int { return bytes.Compare(a[:], b[:]) })

	return out
}

/*
resolve reads who the body mentions among the workspace's teammates and
teams.
*/
func (s *session) resolve(ctx context.Context, reg *ids.Registry, body string) (mentioned, error) {
	var memberIDs, teamIDs []ids.UUID

	for _, token := range mentions.Parse(body) {
		if token.Kind == mentions.KindTeam && s.m.Workspace.TeamMentionsOn() {
			if id, err := reg.Decode(string(models.PrefixTeam), token.ID); err == nil {
				teamIDs = append(teamIDs, id)
			}

			continue
		}

		if id, err := reg.Decode(string(models.PrefixMember), token.ID); err == nil {
			memberIDs = append(memberIDs, id)
		}
	}

	out := mentioned{members: map[ids.UUID]*ids.UUID{}, tokens: []mentionToken{}}

	if err := s.mentionTeams(ctx, teamIDs, &out); err != nil {
		return out, err
	}

	err := s.mentionMembers(ctx, memberIDs, &out)

	return out, err
}

/*
mentionMembers adds the members named directly, a direct mention
winning over one through a team.
*/
func (s *session) mentionMembers(ctx context.Context, memberIDs []ids.UUID, out *mentioned) error {
	if len(memberIDs) == 0 {
		return nil
	}

	members, err := s.w.Q.FindMentionedMembers(ctx, models.FindMentionedMembersParams{
		WorkspaceID: s.c.WorkspaceID, Ids: memberIDs,
	})
	if err != nil {
		return fmt.Errorf("find mentioned members: %w", err)
	}

	for _, member := range members {
		out.tokens = append(out.tokens, mentionToken{
			Kind: mentions.KindMember, ID: models.PublicID(models.PrefixMember, member.ID), Name: member.Name,
		})

		if member.ID != s.m.Member.ID {
			out.members[member.ID] = nil
		}
	}

	return nil
}

/*
mentionTeams adds each active member of the teams named.
*/
func (s *session) mentionTeams(ctx context.Context, teamIDs []ids.UUID, out *mentioned) error {
	if len(teamIDs) == 0 {
		return nil
	}

	teams, err := s.w.Q.FindMentionedTeams(ctx, models.FindMentionedTeamsParams{
		WorkspaceID: s.c.WorkspaceID, Ids: teamIDs,
	})
	if err != nil {
		return fmt.Errorf("find mentioned teams: %w", err)
	}

	live := make([]ids.UUID, 0, len(teams))
	for _, team := range teams {
		live = append(live, team.ID)
		out.tokens = append(out.tokens, mentionToken{
			Kind: mentions.KindTeam, ID: models.PublicID(models.PrefixTeam, team.ID), Name: team.Name,
		})
	}

	members, err := s.w.Q.ListActiveTeamMembers(ctx, models.ListActiveTeamMembersParams{
		WorkspaceID: s.c.WorkspaceID, TeamIds: live,
	})
	if err != nil {
		return fmt.Errorf("list team members: %w", err)
	}

	for _, row := range members {
		if _, taken := out.members[row.WorkspaceMemberID]; !taken && row.WorkspaceMemberID != s.m.Member.ID {
			out.members[row.WorkspaceMemberID] = new(row.TeamID)
		}
	}

	return nil
}

/*
record writes the note's mention rows, drops those of members it no
longer mentions, and returns the members newly mentioned.
*/
func (s *session) record(ctx context.Context, m mentioned) ([]ids.UUID, error) {
	var added []ids.UUID

	for _, member := range m.ids() {
		n, err := s.w.Q.AddMessageMention(ctx, models.AddMessageMentionParams{
			WorkspaceID: s.c.WorkspaceID, MessageID: s.msg.ID, ConversationID: s.c.ID, MemberID: member,
			ViaTeamID: m.members[member], Now: s.w.Now,
		})
		if err != nil {
			return nil, fmt.Errorf("record mention: %w", err)
		}

		if n > 0 {
			added = append(added, member)
		}
	}

	if err := s.w.Q.RemoveMessageMentionsExcept(ctx, models.RemoveMessageMentionsExceptParams{
		WorkspaceID: s.c.WorkspaceID, MessageID: s.msg.ID, Keep: m.ids(),
	}); err != nil {
		return nil, fmt.Errorf("drop old mentions: %w", err)
	}

	return added, nil
}
