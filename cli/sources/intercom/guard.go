package intercom

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
A record is applied only once every record it names has arrived; one
naming a record that never comes waits forever. Teammates, teams and
tags can't be fetched once Intercom deletes them, yet old conversations,
articles and notes still name them. The guard drops those names before
records leave: a removed teammate's message keeps their name as its
label, and a deleted tag or team is left out.
*/

/*
names is who and what dimmi will hold: every teammate, team and tag a
record may name.
*/
type names struct {
	teammates, teams, tags map[string]bool
}

/*
guard drops from the records every teammate, team and tag dimmi won't
hold.
*/
func (s *Source) guard(ctx context.Context, recs []format.Record) ([]format.Record, error) {
	n, err := s.names(ctx, recs)
	if err != nil {
		return nil, err
	}

	for i, r := range recs {
		if r.Kind == format.KindTeammate || r.Kind == format.KindTeam || r.Kind == format.KindTag {
			continue
		}

		raw, err := json.Marshal(r.Data)
		if err != nil {
			return nil, err
		}

		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}

		if !n.clean(v, "") {
			continue
		}

		out, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		recs[i].Data = json.RawMessage(out)
	}

	return recs, nil
}

/*
names reads the teammates, teams and tags Intercom lists, once, with the
former teammates sent so far and those in the records.
*/
func (s *Source) names(ctx context.Context, recs []format.Record) (names, error) {
	known, err := s.knownAdmins(ctx)
	if err != nil {
		return names{}, err
	}

	teams, tags, err := s.knownGroups(ctx)
	if err != nil {
		return names{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	n := names{teammates: map[string]bool{}, teams: teams, tags: tags}

	for id := range known {
		n.teammates[id] = true
	}

	for id, f := range s.former {
		if f.Email != "" {
			n.teammates[id] = true
		}
	}

	for _, r := range recs {
		if r.Kind == format.KindTeammate {
			n.teammates[r.SourceID] = true
		}
	}

	return n, nil
}

/*
knownGroups reads the ids of the teams and tags Intercom lists, once.
*/
func (s *Source) knownGroups(ctx context.Context) (map[string]bool, map[string]bool, error) {
	s.mu.Lock()
	teams, tags := s.knownTeams, s.knownTags
	s.mu.Unlock()

	if teams != nil && tags != nil {
		return teams, tags, nil
	}

	var t struct {
		Teams []team `json:"teams"`
	}

	if err := s.Client.get(ctx, "/teams", nil, &t); err != nil {
		return nil, nil, err
	}

	var g list[tag]
	if err := s.Client.get(ctx, "/tags", nil, &g); err != nil {
		return nil, nil, err
	}

	teams, tags = map[string]bool{}, map[string]bool{}

	for _, x := range t.Teams {
		teams[string(x.ID)] = true
	}

	for _, x := range g.Data {
		tags[string(x.ID)] = true
	}

	s.mu.Lock()
	s.knownTeams, s.knownTags = teams, tags
	s.mu.Unlock()

	return teams, tags, nil
}

/*
clean drops, in place, the names dimmi won't hold, and reports whether
it dropped any.
*/
func (n names) clean(v any, key string) bool {
	changed := false

	switch x := v.(type) {
	case map[string]any:
		if key == "author" || key == "sender" {
			changed = n.cleanAuthor(x)
		}

		for k, child := range x {
			if held, ok := n.held(k); ok {
				changed = dropRefs(x, k, held) || changed

				continue
			}

			changed = n.clean(child, k) || changed
		}
	case []any:
		for _, child := range x {
			changed = n.clean(child, key) || changed
		}
	}

	return changed
}

/*
held gives, for a field that names a teammate, a team or a tag, the ids
dimmi will hold of that kind.
*/
func (n names) held(key string) (map[string]bool, bool) {
	switch key {
	case "assignee_ref", "teammate_ref", "owner_ref", "member_refs":
		return n.teammates, true
	case "team_ref", "team_refs":
		return n.teams, true
	case "tag_ref", "tag_refs", "added_tag_refs", "removed_tag_refs":
		return n.tags, true
	default:
		return nil, false
	}
}

/*
cleanAuthor turns an author who is a teammate dimmi won't hold into the
system, labeled with their name.
*/
func (n names) cleanAuthor(a map[string]any) bool {
	ref, _ := a["ref"].(string)
	if a["type"] != format.AuthorTeammate || ref == "" || n.teammates[ref] {
		return false
	}

	a["type"] = format.AuthorSystem
	delete(a, "ref")

	return true
}

/*
dropRefs removes from the field the ids not in held: a single id is
removed with its field, a list keeps the ids held. An empty id, which
means none, stays.
*/
func dropRefs(m map[string]any, key string, held map[string]bool) bool {
	switch x := m[key].(type) {
	case string:
		if x != "" && !held[x] {
			delete(m, key)

			return true
		}
	case []any:
		kept := slices.DeleteFunc(slices.Clone(x), func(id any) bool {
			s, _ := id.(string)

			return s != "" && !held[s]
		})

		if len(kept) != len(x) {
			m[key] = kept

			return true
		}
	}

	return false
}

/*
Fetch reads, by id, the contacts, companies, conversations and tickets
dimmi waits on that the lists didn't give: an archived contact a
conversation names, say. Ids Intercom doesn't find are left out.
*/
func (s *Source) Fetch(ctx context.Context, kind format.Kind, ids []string) ([]format.Record, error) {
	known, err := s.knownAdmins(ctx)
	if err != nil {
		return nil, err
	}

	fetch := s.fetcher(kind, known)

	if fetch == nil {
		return nil, nil
	}

	recs, err := parallel(ctx, s.Workers, ids, func(ctx context.Context, id string) ([]format.Record, error) {
		recs, err := fetch(ctx, id)
		if isNotFound(err) {
			return nil, nil
		}

		return recs, err
	})
	if err != nil {
		return nil, err
	}

	return s.guard(ctx, recs)
}

/*
fetchOne reads the records of one id.
*/
type fetchOne = func(ctx context.Context, id string) ([]format.Record, error)

/*
fetcher reads one record of the kind by id, or is nil for a kind dimmi
never waits on from Intercom.
*/
func (s *Source) fetcher(kind format.Kind, known map[string]bool) fetchOne {
	switch kind {
	case format.KindContact:
		return func(ctx context.Context, id string) ([]format.Record, error) {
			var c contact
			if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(id), nil, &c); err != nil {
				return nil, err
			}

			return s.fullContact(ctx, c)
		}
	case format.KindCompany:
		return func(ctx context.Context, id string) ([]format.Record, error) {
			var c company
			if err := s.Client.get(ctx, "/companies/"+url.PathEscape(id), nil, &c); err != nil {
				return nil, err
			}

			return []format.Record{translateCompany(c)}, nil
		}
	case format.KindConversation:
		return func(ctx context.Context, id string) ([]format.Record, error) {
			return s.conversationRecords(ctx, ID(id), known)
		}
	case format.KindTicket:
		return func(ctx context.Context, id string) ([]format.Record, error) {
			return s.ticketRecords(ctx, ID(id), known)
		}
	case format.KindTeammate, format.KindTeam, format.KindTag, format.KindAttributeDefinition,
		format.KindContactNote, format.KindTicketType, format.KindHelpCenter, format.KindCollection,
		format.KindArticle, format.KindSnippet, format.KindSavedReply, format.KindSchedule, format.KindWorkflow:
		return nil
	}

	return nil
}

var _ engine.Fetcher = (*Source)(nil)
