/*
|--------------------------------------------------------------------------
| Review
|--------------------------------------------------------------------------
|
| The model's last text is checked in Go before a customer reads it.
| Citation markers become the message's sources and leave the text;
| only results a search of this turn returned count. Untrusted links,
| images and HTML go. A reply cut off by the token limit is trimmed to
| its last full sentence, never left dangling. When the turn's rules
| demand a handoff the agent did not make, code makes it. The outcome
| comes from what the turn did: a handoff when one was made, an answer
| when it cited what it found, small talk when it never searched, and
| otherwise the honest no-answer.
|
*/

package pipeline

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/linkguard"
)

/*
maxReplyChars caps a reply; a longer one is cut at a paragraph.
*/
const maxReplyChars = 6000

/*
review turns the model's draft into the turn's decision.
*/
func (r *Runner) review(ctx context.Context, st *run, draft string) *decision {
	text, cited := citations(draft, st.sources)
	text = r.guard.Filter(text, r.allowedHosts(ctx, st))
	if st.loop != nil && st.loop.stop == contracts.StopReasonMaxTokens {
		text = trimToSentence(text)
	}
	text = capLength(text)

	if enforceRules(st) {
		text = strings.TrimSpace(text + "\n\n" + r.line(st.env, st.env.handoffKey()))
	}

	switch {
	case st.handoff != nil:
		d := handoff(st.handoff.reason, text, st.env.handoffKey())
		d.citations = cited
		return d
	case len(cited) > 0:
		return &decision{outcome: outcomeAnswered, reply: text, citations: cited}
	case !st.searched && text != "":
		return &decision{outcome: outcomeGreeted, reply: text}
	}
	return unknown(text)
}

/*
markerPattern matches citation markers: [1], [1, 3], [1][3].
*/
func markerPattern() *regexp.Regexp {
	return regexp.MustCompile(`[ \t]?\[(\d{1,3}(?:\s*[,;]\s*\d{1,3})*)\]`)
}

/*
citations strips every marker from the text and returns the supplied
sources the valid ones cite, in first-cited order, one per document:
several chunks of one article are one source to the customer. A marker
followed by "(" is a Markdown link and stays.
*/
func citations(text string, sources []source) (string, []citation) {
	var cited []citation
	var b strings.Builder
	last := 0
	for _, loc := range markerPattern().FindAllStringSubmatchIndex(text, -1) {
		if loc[1] < len(text) && text[loc[1]] == '(' {
			continue
		}
		b.WriteString(text[last:loc[0]])
		last = loc[1]
		for _, part := range strings.FieldsFunc(text[loc[2]:loc[3]], func(r rune) bool {
			return r == ',' || r == ';' || r == ' '
		}) {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(sources) {
				continue
			}
			c := citationOf(sources[n-1])
			if !slices.ContainsFunc(cited, func(o citation) bool { return o.DocumentID == c.DocumentID }) {
				cited = append(cited, c)
			}
		}
	}
	b.WriteString(text[last:])
	return strings.TrimSpace(b.String()), cited
}

/*
citationOf is the metadata entry for a cited source.
*/
func citationOf(s source) citation {
	return citation{
		N: s.n, ChunkID: s.chunk.ID.String(), DocumentID: s.chunk.DocumentID.String(),
		Title: s.chunk.Title, URL: s.chunk.URL, Similarity: similarityOf(s.chunk.Similarity),
	}
}

/*
allowedHosts are the hosts a reply may link to: the sources' own, the
help center's and the agent's allowlist.
*/
func (r *Runner) allowedHosts(ctx context.Context, st *run) []string {
	urls := make([]string, 0, len(st.sources))
	for _, s := range st.sources {
		urls = append(urls, s.chunk.URL)
	}
	hosts := append(linkguard.Hosts(urls), st.env.Agent.LinkAllowlist...)
	if r.Knowledge != nil {
		if public, err := r.Knowledge.PublicHosts(ctx, st.env.Workspace.ID); err == nil {
			hosts = append(hosts, public...)
		}
	}
	return hosts
}

/*
sentenceEnd matches the end of a full sentence.
*/
func sentenceEnd() *regexp.Regexp {
	return regexp.MustCompile(`[.!?。！？](?:["')\]*_]*)(?:\s|$)`)
}

/*
trimToSentence keeps the text up to its last full sentence; with none,
nothing is left.
*/
func trimToSentence(text string) string {
	locs := sentenceEnd().FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return ""
	}
	return strings.TrimSpace(text[:locs[len(locs)-1][1]])
}

/*
capLength cuts an overlong reply at its last paragraph break before
the cap.
*/
func capLength(text string) string {
	if len(text) <= maxReplyChars {
		return text
	}
	cut := text[:maxReplyChars]
	if i := strings.LastIndex(cut, "\n\n"); i > 0 {
		return strings.TrimSpace(cut[:i])
	}
	return trimToSentence(cut)
}
