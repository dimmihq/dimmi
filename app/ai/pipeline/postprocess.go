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
| otherwise the honest no-answer. A reply grounded in an action's result
| or guided by one of the team's procedures is an answer too, and a
| reply saying a request now waits for the customer or the team is not
| a failed answer.
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
	draft = strings.TrimSpace(earlierMarker().ReplaceAllString(earlierSources().ReplaceAllString(draft, ""), ""))
	text, cited := citations(draft, st.sources)
	hosts := r.allowedHosts(ctx, st)
	text = r.guard.Filter(text, hosts)
	st.annotated = annotate(r.guard.Filter(draft, hosts), st.sources)
	st.citedChunks = citedChunks(draft, st.sources)
	if st.loop != nil && st.loop.stop == contracts.StopReasonMaxTokens {
		text = trimToSentence(text)
	}
	text = capLength(text)

	if st.handoff == nil && !st.actions.involved() && claimsHandoff(text) {
		st.handoff = &handoffRequest{reason: "other"}
		st.flags = append(st.flags, "implied_handoff")
	}
	if enforceRules(st) {
		text = strings.TrimSpace(text + "\n\n" + r.line(st.env, st.env.handoffKey()))
	}

	switch {
	case st.handoff != nil:
		d := handoff(st.handoff.reason, text, st.env.handoffKey())
		d.citations = cited
		return d
	case len(cited) > 0, (st.actions.answered || st.followed) && text != "":
		return &decision{outcome: outcomeAnswered, reply: text, citations: cited}
	case st.actions.waiting && text != "":
		return &decision{outcome: outcomeGreeted, reply: text}
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
earlierMarker matches the markers earlier replies carry in the history,
which a reply must never repeat.
*/
func earlierMarker() *regexp.Regexp {
	return regexp.MustCompile(`[ \t]?\[S\d{1,3}(?:\s*[,;]\s*S\d{1,3})*\]`)
}

/*
citedChunks are the passages the draft's markers name, in first-cited
order, each once.
*/
func citedChunks(draft string, sources []source) []source {
	var out []source
	for _, m := range markerPattern().FindAllStringSubmatch(draft, -1) {
		for _, part := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(sources) {
				continue
			}
			if !slices.ContainsFunc(out, func(s source) bool { return s.chunk.ID == sources[n-1].chunk.ID }) {
				out = append(out, sources[n-1])
			}
		}
	}
	return out
}

/*
involved reports whether the turn requested, reported or awaits an
action, where a reply rightly says a teammate has the request.
*/
func (a actionState) involved() bool {
	return len(a.calls) > 0 || a.card != nil || a.followUp != nil || a.waiting || a.answered
}

/*
claimsHandoff reports whether a reply says the agent passed the
conversation to the team, asked or told a teammate, or the like. A reply
that says so without the handoff tool is made true: the turn hands off.
*/
func claimsHandoff(text string) bool {
	return handoffClaim().MatchString(text)
}

/*
handoffClaim matches "I've passed this to the billing team", "I've asked
the team to confirm", "I have flagged it for someone" and the like.
*/
func handoffClaim() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\bI(?:['’]ve| have) (?:just |now |already )?(?:passed|handed|sent|forwarded|flagged|asked|told|let|brought in|looped in|routed)\b[^.!?\n]{0,80}?\b(?:team|teammate|colleague|someone|somebody|billing|support|specialist)s?\b`)
}

/*
earlierSources matches the list of documents earlier replies open with in
the history, when the model copies it into a reply: a <cited> block, or
lines that name a document by its [S] marker.
*/
func earlierSources() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<cited>.*?</cited>|(?m)^[ \t]*\[S\d{1,3}\][^\n]*$`)
}

/*
annotate is the reply as later turns show it to the model: each marker
rewritten as [S1], [S2] by the documents the reply cites, opened by the
list of those documents. A reply that cites nothing is empty.
*/
func annotate(draft string, sources []source) string {
	var docs []source
	var b strings.Builder
	last := 0
	for _, loc := range markerPattern().FindAllStringSubmatchIndex(draft, -1) {
		if loc[1] < len(draft) && draft[loc[1]] == '(' {
			continue
		}
		b.WriteString(draft[last:loc[0]])
		last = loc[1]
		var tags []string
		for _, part := range strings.FieldsFunc(draft[loc[2]:loc[3]], func(r rune) bool {
			return r == ',' || r == ';' || r == ' '
		}) {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(sources) {
				continue
			}
			k := slices.IndexFunc(docs, func(d source) bool { return d.chunk.DocumentID == sources[n-1].chunk.DocumentID })
			if k < 0 {
				docs = append(docs, sources[n-1])
				k = len(docs) - 1
			}
			tag := "S" + strconv.Itoa(k+1)
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
		if len(tags) > 0 {
			b.WriteString(" [" + strings.Join(tags, ", ") + "]")
		}
	}
	if len(docs) == 0 {
		return ""
	}
	b.WriteString(draft[last:])
	var head strings.Builder
	head.WriteString("<cited>")
	for i, d := range docs {
		if i > 0 {
			head.WriteString("; ")
		}
		head.WriteString("[S" + strconv.Itoa(i+1) + "] " + d.chunk.Title)
	}
	head.WriteString("</cited>\n")
	return head.String() + strings.TrimSpace(b.String())
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
links the sources quote, the help center's and the agent's allowlist.
*/
func (r *Runner) allowedHosts(ctx context.Context, st *run) []string {
	urls := make([]string, 0, len(st.sources))
	for _, s := range st.sources {
		urls = append(append(urls, s.chunk.URL), r.guard.Find(s.chunk.Text)...)
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
