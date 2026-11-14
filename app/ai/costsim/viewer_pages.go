/*
|--------------------------------------------------------------------------
| Run Viewer Pages
|--------------------------------------------------------------------------
|
| The viewer's three pages, written as plain HTML with every value
| escaped: the index of runs, a run, and a conversation.
|
*/

package costsim

import (
	"fmt"
	"html"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
page builds one page.
*/
type page struct {
	b strings.Builder
}

/*
raw writes markup as it is.
*/
func (p *page) raw(s string) *page {
	p.b.WriteString(s)
	return p
}

/*
text writes a value, escaped.
*/
func (p *page) text(v any) *page {
	p.b.WriteString(html.EscapeString(fmt.Sprint(v)))
	return p
}

/*
row writes a table row of escaped cells; a markup cell is written as
it is, and a number is right-aligned.
*/
func (p *page) row(cells ...any) {
	p.raw("<tr>")
	for _, c := range cells {
		switch v := c.(type) {
		case markup:
			p.raw("<td>").raw(string(v)).raw("</td>")
		case int, int32, int64, float64:
			p.raw(`<td class="n">`).text(v).raw("</td>")
		default:
			p.raw("<td>").text(v).raw("</td>")
		}
	}
	p.raw("</tr>\n")
}

/*
head writes a table's header row.
*/
func (p *page) head(cells ...string) {
	p.raw("<table><tr>")
	for _, c := range cells {
		p.raw("<th>").text(c).raw("</th>")
	}
	p.raw("</tr>\n")
}

/*
markup is a cell already written as HTML.
*/
type markup string

/*
link is an escaped link.
*/
func link(href, label string) markup {
	return markup(`<a href="` + html.EscapeString(href) + `">` + html.EscapeString(label) + `</a>`)
}

/*
open starts a page with its title.
*/
func open(title string) *page {
	p := &page{}
	p.raw(`<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>`).text(title).
		raw("</title>").raw(viewerStyle).raw("</head><body>\n")
	return p
}

/*
close ends a page.
*/
func (p *page) close() string {
	p.raw("</body></html>\n")
	return p.b.String()
}

/*
viewerStyle is the pages' look: plain, readable, light and dark.
*/
const viewerStyle = `<style>
:root{color-scheme:light dark;--bg:#fff;--fg:#1a1a1a;--muted:#666;--line:#ddd;--card:#f6f6f6}
@media (prefers-color-scheme:dark){:root{--bg:#141414;--fg:#e8e8e8;--muted:#999;--line:#333;--card:#1e1e1e}}
body{background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif;margin:0;padding:16px 24px}
a{color:#3b82f6}h1{font-size:20px}h2{font-size:16px;margin-top:28px}.muted{color:var(--muted)}
table{border-collapse:collapse;width:100%;margin:8px 0}
th,td{border-bottom:1px solid var(--line);padding:5px 8px;text-align:left;vertical-align:top}
th{color:var(--muted);font-weight:500}td.n{text-align:right;font-variant-numeric:tabular-nums}
.msg{background:var(--card);border-radius:8px;padding:8px 12px;margin:6px 0;white-space:pre-wrap}
.contact{border-left:3px solid #3b82f6}.agent{border-left:3px solid #10b981}.note{border-left:3px solid #f59e0b}
pre{white-space:pre-wrap;background:var(--card);padding:8px;border-radius:6px;max-height:420px;overflow:auto;
font-size:12px}
</style>`

/*
when writes a time in UTC.
*/
func when(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04")
}

/*
pct writes a share as a percentage.
*/
func pct(f float64) string {
	return strconv.FormatFloat(f*100, 'f', 0, 64) + "%"
}

/*
usd writes micro dollars as dollars.
*/
func usd(m int64) string {
	return fmt.Sprintf("$%.4f", float64(m)/1e6)
}

/*
indexHTML is the list of every run.
*/
func indexHTML(runs []viewRun, built time.Time) string {
	p := open("Benchmark runs")
	p.raw("<h1>Benchmark runs</h1><p class=\"muted\">").text(fmt.Sprintf("%d runs, built %s UTC. ", len(runs),
		when(built))).raw("Rebuild with <code>dimmi ai:runs-viewer</code>.</p>\n")
	p.head("Started", "Run", "Kind", "Model", "n", "Spend", "Mean / conv", "p95", "Cache hit", "Source")
	for _, r := range runs {
		o := r.Report.Overall
		p.row(when(r.Report.Started), link("run-"+r.Slug+".html", r.Slug), r.Kind, r.Report.AgentModel,
			len(r.Report.Conversations), usd(r.Report.SpendMicros), cents(o.PerConversation.Mean),
			cents(float64(o.PerConversation.P95)), pct(o.CacheHitShare), markup(`<span class="muted">`+
				html.EscapeString(r.Source)+`</span>`))
	}
	return p.raw("</table>").close()
}

/*
runHTML is one run's page.
*/
func runHTML(r viewRun) string {
	rep := r.Report
	p := open(r.Slug)
	p.raw(`<p><a href="index.html">All runs</a></p><h1>`).text(r.Slug).raw("</h1><p>").
		text(fmt.Sprintf("%s run on %s, started %s UTC, %d conversations, spend %s. Model calls over 100K tokens: %d.",
			r.Kind, rep.AgentModel, when(rep.Started), len(rep.Conversations), usd(rep.SpendMicros), rep.BigPrompts))
	if rep.StoppedBy != "" {
		p.text(" Stopped by " + rep.StoppedBy + ".")
	}
	p.raw("</p>\n")
	if q := r.Quality; q != nil {
		p.raw("<p>").text(fmt.Sprintf("Quality: retrieval hit %s, average score %.2f, correct %s, contradicting %d, "+
			"failed %d, judge %s.", pct(q.HitRate), q.AvgScore, pct(q.CorrectShare), q.Contradicting, q.Failed,
			q.JudgeModel)).raw("</p>\n")
	}
	p.raw("<h2>Cost per conversation</h2>")
	p.head("Slice", "n", "Mean", "Median", "p90", "p95", "p99", "Max", "Per AI reply", "Cache hit", "Turns", "Replies")
	groupRow(p, "all", rep.Overall)
	for _, b := range []string{"1", "2-3", "4-7", "8+"} {
		if g, ok := rep.ByReplies[b]; ok && g.PerConversation.Count > 0 {
			groupRow(p, b+" AI replies", g)
		}
	}
	p.raw("</table><h2>Conversations</h2>")
	p.head("#", "Scenario", "Kind", "Turns", "Replies", "Calls", "Fresh in", "Cached in", "Out", "Cost", "Outcome")
	for i, c := range rep.Conversations {
		p.row(link(fmt.Sprintf("conv-%s-%d.html", r.Slug, i+1), strconv.Itoa(i+1)), c.Scenario, c.Kind, c.Turns,
			c.AIReplies, c.ModelCalls, c.NewTokens, c.CachedTokens, c.OutputTokens, cents(float64(c.CostMicros)),
			strings.TrimSpace(c.Outcome+" "+c.Error))
	}
	p.raw("</table><h2>Settings</h2><pre>").text(jsonText(rep.Options)).raw("</pre>")
	return p.close()
}

/*
groupRow writes one slice's figures.
*/
func groupRow(p *page, label string, g Group) {
	c := g.PerConversation
	p.row(label, c.Count, cents(c.Mean), cents(float64(c.Median)), cents(float64(c.P90)), cents(float64(c.P95)),
		cents(float64(c.P99)), cents(float64(c.Max)), cents(g.PerReply.Mean), pct(g.CacheHitShare),
		fmt.Sprintf("%.1f", g.AvgTurns), fmt.Sprintf("%.1f", g.AvgReplies))
}

/*
convHTML is one conversation's page.
*/
func convHTML(v convView) string {
	c := v.Result
	p := open(c.Scenario)
	p.raw(`<p><a href="index.html">All runs</a> / `).raw(string(link("run-"+v.Run.Slug+".html", v.Run.Slug))).
		raw("</p><h1>").text(fmt.Sprintf("%d. %s", v.N, c.Scenario)).raw("</h1><p>").
		text(fmt.Sprintf("%s, %d turns, %d AI replies, %d model calls, %s, outcome %s %s", c.Kind, c.Turns,
			c.AIReplies, c.ModelCalls, cents(float64(c.CostMicros)), c.Outcome, c.Error)).raw("</p>\n")
	if q := v.Quality; q != nil {
		p.raw("<p>").text(fmt.Sprintf("Judge: score %d, correct %v, contradicts %v, retrieval hit %v. %s",
			q.Grade.Score, q.Grade.Correct, q.Grade.Contradicts, q.Hit, q.Grade.Reason)).raw("</p>\n")
	}
	if v.Missing {
		p.raw(`<p class="muted">The database no longer holds this conversation (re-seeded since); ` +
			`the figures above come from the report.</p>`)
	}
	if len(v.Messages) > 0 {
		p.raw("<h2>Transcript</h2>")
		for _, m := range v.Messages {
			p.raw(`<div class="msg `).text(m.Author + " " + m.Type).raw(`"><span class="muted">`).
				text(fmt.Sprintf("%s %s · %s", m.Author, m.Type, when(m.At))).raw("</span>\n").text(m.Body).
				raw("</div>")
		}
	}
	ledgerTable(p, c.Ledger)
	for _, r := range v.Runs {
		runTable(p, r)
	}
	return p.close()
}

/*
ledgerTable writes a conversation's token ledger.
*/
func ledgerTable(p *page, ledger map[string]PartCost) {
	if len(ledger) == 0 {
		return
	}
	p.raw("<h2>Token ledger</h2>")
	p.head("Part", "Cached", "Written", "Fresh", "Cost")
	for _, part := range slices.Sorted(maps.Keys(ledger)) {
		l := ledger[part]
		p.row(part, l.Cached, l.Written, l.Fresh, cents(float64(l.Micros)))
	}
	p.raw("</table>")
}

/*
runTable writes one AI run's steps with their requests and responses.
*/
func runTable(p *page, r viewAIRun) {
	p.raw("<h2>").text(fmt.Sprintf("AI run %s: %s %s, %s", r.ID, r.Status, r.Outcome, cents(float64(r.Cost)))).
		raw("</h2>")
	p.head("#", "Step", "Model / tool", "Status", "Fresh", "Cached", "Written", "Out", "Cost", "ms")
	for _, s := range r.Steps {
		p.row(s.Seq, s.Kind, s.Model, s.Status, s.Fresh, s.Cached, s.Written, s.Output, cents(float64(s.Cost)),
			s.Latency)
		p.raw(`<tr><td></td><td colspan="9"><details><summary>request</summary><pre>`).text(s.Request).
			raw(`</pre></details><details><summary>response</summary><pre>`).text(s.Response).
			raw("</pre></details></td></tr>\n")
	}
	p.raw("</table>")
}

/*
jsonText writes a value as indented JSON.
*/
func jsonText(v any) string {
	out, err := jsonIndent(v)
	if err != nil {
		return ""
	}
	return out
}
