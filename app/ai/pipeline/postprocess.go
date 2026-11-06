/*
|--------------------------------------------------------------------------
| Post-Process
|--------------------------------------------------------------------------
|
| The model's reply is checked in Go before a customer reads it.
| Citation markers become citations on the message and leave the text;
| only sources the run actually supplied count. Untrusted links, images
| and HTML go. A reply cut off by the token limit is trimmed to its last
| full sentence, never left dangling. An answer that cites nothing when
| citations are required, or that a strict verifier finds unsupported,
| becomes the honest no-answer.
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
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/linkguard"
)

/*
maxReplyChars caps a reply; a longer one is cut at a paragraph.
*/
const maxReplyChars = 6000

/*
postProcess turns a parsed reply into the run's decision.
*/
func (r *Runner) postProcess(ctx context.Context, st *run, p parsed) *decision {
	text, cited := citations(p.reply, st.sources)
	text = r.guard.Filter(text, r.allowedHosts(ctx, st))
	if p.stop == contracts.StopReasonMaxTokens {
		text = trimToSentence(text)
	}
	text = capLength(text)

	switch p.status {
	case outcomeUnknown:
		return unknown(text)
	case outcomeHandoff:
		return handoff(handoffReason(st), text, st.env.handoffKey())
	}

	return r.answered(ctx, st, text, cited)
}

/*
parsed is a reply split by its STATUS line, with why the stream ended.
*/
type parsed struct {
	status, reply string
	stop          contracts.StopReason
}

/*
answered applies the answer-only rules: something left to say, a
citation when required, the verifier in strict mode, and a person
when the customer also asked for one.
*/
func (r *Runner) answered(ctx context.Context, st *run, text string, cited []citation) *decision {
	e := st.env
	switch {
	case text == "":
		return unknown("")
	case st.noSources && st.c.Intent == "greeting":
		return &decision{outcome: outcomeGreeted, reply: text}
	case st.noSources:
		return &decision{outcome: outcomeRefused, reply: text}
	case e.Agent.RequireCitations && len(cited) == 0:
		st.flags = append(st.flags, "uncited")
		return unknown("")
	case strictGrounding(e) && !r.verify(ctx, st, text):
		st.flags = append(st.flags, "unsupported")
		return unknown("")
	case st.c.WantsHuman:
		line := r.line(e, e.handoffKey())
		return &decision{
			outcome: outcomeHandoff, handoffReason: "customer_requested",
			reply: text + "\n\n" + line, citations: cited,
		}
	}
	return &decision{outcome: outcomeAnswered, reply: text, citations: cited}
}

/*
handoffReason names why the answer model handed off.
*/
func handoffReason(st *run) string {
	switch {
	case st.c.WantsHuman || st.c.Intent == "human_request":
		return "customer_requested"
	case st.c.Intent == "unsafe":
		return "safety"
	case st.c.Intent == "abusive":
		return "abusive"
	case st.c.Frustrated:
		return "frustration"
	}
	return "needs_human"
}

/*
markerPattern matches citation markers: [1], [1, 3], [1][3].
*/
func markerPattern() *regexp.Regexp {
	return regexp.MustCompile(`[ \t]?\[(\d{1,3}(?:\s*[,;]\s*\d{1,3})*)\]`)
}

/*
citations strips every marker from the text and returns the supplied
sources the valid ones cite, in first-cited order. A marker followed
by "(" is a Markdown link and stays.
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
			if !slices.ContainsFunc(cited, func(c citation) bool { return c.N == n }) {
				cited = append(cited, citationOf(sources[n-1]))
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

/*
strictGrounding reports whether every answer goes past the verifier.
*/
func strictGrounding(e *env) bool {
	return e.Agent.GroundingMode == "strict" || e.Agent.GroundingMode == "verify"
}

/*
verdict is the verifier's answer.
*/
type verdict struct {
	Supported        *bool    `json:"supported"`
	UnsupportedClaim []string `json:"unsupported_claims"`
}

/*
validVerdict accepts a verdict that says whether the draft holds.
*/
func validVerdict(v any) bool {
	out, ok := v.(*verdict)
	return ok && out.Supported != nil
}

/*
verify asks the answer provider's cheap model whether the sources support every
claim of the draft. A verifier that fails to answer fails closed.
*/
func (r *Runner) verify(ctx context.Context, st *run, draft string) bool {
	e := st.env
	llm, model, key := r.cheapLLM(ctx, e)
	if llm == nil {
		return false
	}

	var out verdict
	ok := r.callJSON(ctx, st, jsonCall{
		kind: enums.AIRunStepKindVerify, llm: llm, key: key, model: model,
		system:  r.render(e, "verify.system.md", nil),
		user:    "<draft>" + escape(draft) + "</draft>\n" + sourcesBlock(st.sources),
		timeout: r.Timeouts.VerifyTotal, maxTokens: 500, valid: validVerdict,
	}, &out)

	return ok && *out.Supported
}
