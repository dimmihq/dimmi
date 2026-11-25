/*
|--------------------------------------------------------------------------
| Related Conversations
|--------------------------------------------------------------------------
|
| To recommend who should take a conversation, how urgent it is or how
| to tag it, Operator needs more than the conversation in front of it:
| it needs what the team did with the ones like it. This tool finds past
| conversations on a topic and says, for each, who handled it, in which
| team, under which tags and priority, how it ended and how long it
| took, then counts who handled the most. A recommendation can then rest
| on what the team does, and say so.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
relatedLimit is how many related conversations one search shows, and
upFront how many a conversation's page hands Operator without asking.
*/
const (
	relatedLimit = 15
	upFront      = 5
)

/*
relatedTool is the tool that finds related conversations.
*/
func (e *Engine) relatedTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: "find_related_conversations",
		Description: "Find past conversations like one and see what the team did with each: what was asked " +
			"and how it was answered, who handled it, in which team, its tags and priority, how it ended " +
			"and how long it took, with a count of who handled the most. With no topic it finds the ones " +
			"most like the conversation by meaning. Call it before you recommend a teammate, a team, a tag " +
			"or a priority, and to see how the team answered the same question before.",
		Schema: schema(`{"type":"object","properties":{` +
			`"conversation_number":{"type":"integer","description":"The conversation you want relatives of. ` +
			`Leave out for the one that is open."},` +
			`"topic":{"type":"string","maxLength":256,"description":"Leave out to find the conversations most ` +
			`like this one. Give two to four plain words, like 'refund voided label', only to look for ` +
			`another topic."}}}`),
	}, handle: e.findRelated}
}

/*
related is one related conversation: how it stands and what the
workspace remembers of it.
*/
type related struct {
	loaded conversations.Loaded
	text   string
}

/*
findRelated finds conversations like one, or on a topic, and says who
handled each.
*/
func (e *Engine) findRelated(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Topic  string `json:"topic"`
		Number int64  `json:"conversation_number"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	topic := strings.TrimSpace(in.Topic)
	var hits []related
	var err error
	what := "this one"
	if topic == "" {
		hits, err = e.alike(ctx, s, in.Number, relatedLimit)
	} else {
		what = fmt.Sprintf("%q", topic)
		hits, err = e.onTopic(ctx, s, topic, in.Number)
	}
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	st := Step{Tool: "find_related_conversations", Summary: fmt.Sprintf("Looked for conversations like %s: %d found",
		what, len(hits))}
	if len(hits) == 0 {
		return result{text: "No related conversations found. Try a topic of two to four plain words; when " +
			"there are none, say your recommendation is a guess.", step: st}, nil
	}
	return result{text: e.relatedText(s, hits), step: st}, nil
}

/*
alike finds the conversations most like one, by keywords and meaning,
from the workspace's memory.
*/
func (e *Engine) alike(ctx context.Context, s *session, number int64, limit int) ([]related, error) {
	if e.Services.Memory == nil {
		return nil, nil
	}
	r, err := ref(s, number)
	if err != nil {
		return nil, err
	}
	own, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{Membership: s.m, Ref: r})
	if err != nil {
		return nil, err
	}
	return e.alikeTo(ctx, s, own.Conversation.ID, limit)
}

/*
alikeTo finds the conversations most like the one with the id.
*/
func (e *Engine) alikeTo(ctx context.Context, s *session, id ids.UUID, limit int) ([]related, error) {
	if e.Services.Memory == nil {
		return nil, nil
	}
	similar, err := e.Services.Memory.SimilarConversations(ctx, s.m, id, limit)
	if err != nil {
		return nil, err
	}
	var hits []related
	for _, one := range similar {
		loaded, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{
			Membership: s.m, Ref: conversations.Ref{ID: one.ID},
		})
		if err != nil {
			continue
		}
		hits = append(hits, related{loaded: loaded, text: one.Text})
	}
	return hits, nil
}

/*
onTopic finds conversations by the words of a topic, leaving out the
one asked about.
*/
func (e *Engine) onTopic(ctx context.Context, s *session, topic string, number int64) ([]related, error) {
	found, err := e.Services.Search.Handle(ctx, conversations.SearchInput{
		Membership: s.m, Query: topic, Limit: relatedLimit + 1,
	})
	if err != nil {
		return nil, err
	}
	own := number
	if own == 0 && s.conversation != nil {
		own = s.conversation.Number
	}
	var hits []related
	for _, hit := range found.Conversations {
		if hit.Conversation.Conversation.Number != own && len(hits) < relatedLimit {
			hits = append(hits, related{loaded: hit.Conversation})
		}
	}
	return hits, nil
}

/*
relatedText writes related conversations and the count of who handled
them.
*/
func (e *Engine) relatedText(s *session, hits []related) string {
	var b strings.Builder
	loaded := make([]conversations.Loaded, len(hits))
	for i, hit := range hits {
		loaded[i] = hit.loaded
		writeRelated(&b, hit.loaded)
		fmt.Fprintf(&b, "  link %s\n", e.conversationLink(s, hit.loaded.Conversation.ID))
		if text := strings.TrimSpace(hit.text); text != "" {
			fmt.Fprintf(&b, "  what was asked and how it was answered: %s\n", untrusted(truncate(text, 600)))
		}
	}
	b.WriteString("\n" + handlers(loaded))
	return b.String()
}

/*
writeRelated writes one related conversation: what it was, who handled
it and how it went.
*/
func writeRelated(b *strings.Builder, l conversations.Loaded) {
	c := l.Conversation
	fmt.Fprintf(b, "#%d %s\n  %s · priority %s · tags: %s\n  handled by %s · team %s · %s\n", c.Number,
		untrusted(deref(c.Subject)), c.Status, enums.ConversationPriorityOf(c.Priority), tagNames(l.Tags),
		handlerOf(l), teamNameOf(l), howItWent(c.CreatedAt, c.ClosedAt))
}

/*
handlerOf names who handled a conversation: its assignee, else the
teammate who wrote last, else nobody.
*/
func handlerOf(l conversations.Loaded) string {
	switch {
	case l.Assignee != nil:
		return l.Assignee.User.Name
	case l.LastAuthor != nil:
		return l.LastAuthor.User.Name + " (replied last, not assigned)"
	}
	return "nobody"
}

/*
teamNameOf names a conversation's team without its id.
*/
func teamNameOf(l conversations.Loaded) string {
	if l.Team == nil {
		return "none"
	}
	return l.Team.Name
}

/*
howItWent says when a conversation opened and how long it took to
close, or that it is still open.
*/
func howItWent(opened time.Time, closed *time.Time) string {
	day := opened.UTC().Format("2006-01-02")
	if closed == nil {
		return "opened " + day + ", not closed yet"
	}
	took := closed.Sub(opened).Round(time.Hour)
	if took < time.Hour {
		return "opened " + day + ", closed within the hour"
	}
	if took < 48*time.Hour {
		return fmt.Sprintf("opened %s, closed after %d hours", day, int(took.Hours()))
	}
	return fmt.Sprintf("opened %s, closed after %d days", day, int(took.Hours()/24))
}

/*
handlers counts who handled the conversations found, which teams and
which tags, most first, so a recommendation can cite the numbers.
*/
func handlers(hits []conversations.Loaded) string {
	people, teams, tags := map[string]int{}, map[string]int{}, map[string]int{}
	for _, l := range hits {
		if l.Assignee != nil {
			people[l.Assignee.User.Name]++
		}
		if l.Team != nil {
			teams[l.Team.Name]++
		}
		for _, t := range l.Tags {
			tags[t.Name]++
		}
	}
	return fmt.Sprintf("Of these %d conversations:\nAssigned to: %s\nTeams: %s\nMost used tags: %s", len(hits),
		ranked(people), ranked(teams), ranked(tags))
}

/*
ranked lists names by how often they came up, most first, at most
five.
*/
func ranked(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	names = names[:min(len(names), 5)]
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = fmt.Sprintf("%s %d", name, counts[name])
	}
	return strings.Join(out, ", ")
}
