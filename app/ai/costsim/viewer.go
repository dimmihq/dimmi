/*
|--------------------------------------------------------------------------
| Run Viewer
|--------------------------------------------------------------------------
|
| Every benchmark run leaves a report under tmp/. The viewer turns all
| of them into plain pages a person can open at any time: an index of
| runs, a page per run with its settings, spend and figures, and a page
| per conversation with the full transcript and every model call with
| its request, response, tokens and cost, read from the database. A
| conversation the database no longer holds, such as one from before a
| re-seed, says so and keeps its figures from the report.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Viewer builds the pages from the reports one disk holds onto another.
*/
type Viewer struct {
	App  *app.App
	From filesystem.Filesystem
	To   filesystem.Filesystem
}

/*
viewRun is one run as the pages show it.
*/
type viewRun struct {
	Slug    string
	Source  string
	Kind    string
	Report  *Report
	Quality *QualityReport
}

/*
Build writes every page and returns how many runs it found.
*/
func (v *Viewer) Build(ctx context.Context) (int, error) {
	runs, err := v.runs(ctx)
	if err != nil {
		return 0, err
	}
	slices.SortFunc(runs, func(a, b viewRun) int { return b.Report.Started.Compare(a.Report.Started) })
	if err := v.write(ctx, "index.html", indexHTML(runs, v.App.Clock.Now())); err != nil {
		return 0, err
	}
	for _, r := range runs {
		if err := v.write(ctx, "run-"+r.Slug+".html", runHTML(r)); err != nil {
			return 0, err
		}
		for i, c := range r.Report.Conversations {
			page := v.conversation(ctx, r, i, c)
			if err := v.write(ctx, fmt.Sprintf("conv-%s-%d.html", r.Slug, i+1), convHTML(page)); err != nil {
				return 0, err
			}
		}
	}
	return len(runs), nil
}

/*
runs reads every report and quality report on the source disk.
*/
func (v *Viewer) runs(ctx context.Context) ([]viewRun, error) {
	var out []viewRun
	for _, prefix := range ReportFolders() {
		found, err := v.runsUnder(ctx, prefix)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

/*
ReportFolders are the folders under tmp/ that hold run reports: the
benchmark's own, and those of earlier runs.
*/
func ReportFolders() []string {
	return []string{"benchmarks/runs/", "token-optimisation/", "costsim-bench-layer/", "costsim/", "quality/"}
}

/*
runsUnder reads the reports under one folder.
*/
func (v *Viewer) runsUnder(ctx context.Context, prefix string) ([]viewRun, error) {
	var out []viewRun
	for entry, err := range v.From.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		name := path.Base(entry.Path)
		if name != "report.json" && name != "quality.json" {
			continue
		}
		raw, err := v.From.Get(ctx, entry.Path)
		if err != nil {
			return nil, err
		}
		if r, ok := loadRun(entry.Path, name, raw); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

/*
loadRun reads one report, reporting false for one that does not parse.
*/
func loadRun(file, name string, raw []byte) (viewRun, bool) {
	r := viewRun{Source: file, Slug: slugOf(file)}
	if name == "quality.json" {
		var q QualityReport
		if json.Unmarshal(raw, &q) != nil || q.Cost == nil {
			return r, false
		}
		r.Kind, r.Quality, r.Report = "quality", &q, q.Cost
	} else {
		var rep Report
		if json.Unmarshal(raw, &rep) != nil {
			return r, false
		}
		r.Kind, r.Report = runKind(&rep), &rep
	}
	r.Report.summarize()
	return r, true
}

/*
runKind names what a cost run measured.
*/
func runKind(r *Report) string {
	for _, c := range r.Conversations {
		if c.Kind == "operator" {
			return "operator"
		}
	}
	return "customer"
}

/*
slugOf turns a report's path into a page name.
*/
func slugOf(p string) string {
	p = strings.TrimSuffix(strings.TrimSuffix(p, "/report.json"), "/quality.json")
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, p)
}

/*
convView is a conversation page: the run, the conversation's figures,
its transcript and its model calls.
*/
type convView struct {
	Run      viewRun
	N        int
	Result   ConversationResult
	Quality  *QualityResult
	Messages []viewMessage
	Runs     []viewAIRun
	Missing  bool
}

/*
viewMessage is one line of the transcript.
*/
type viewMessage struct {
	At     time.Time
	Author string
	Type   string
	Body   string
}

/*
viewAIRun is one AI run and its steps.
*/
type viewAIRun struct {
	ID      string
	Status  string
	Outcome string
	Cost    int64
	Steps   []viewStep
}

/*
viewStep is one step: a model call or a tool call.
*/
type viewStep struct {
	Seq      int32
	Kind     string
	Model    string
	Status   string
	Fresh    int64
	Cached   int64
	Written  int64
	Output   int64
	Cost     int64
	Latency  int32
	Request  string
	Response string
}

/*
conversation gathers one conversation's page from the database.
*/
func (v *Viewer) conversation(ctx context.Context, r viewRun, i int, c ConversationResult) convView {
	out := convView{Run: r, N: i + 1, Result: c}
	if r.Quality != nil && i < len(r.Quality.Items) {
		out.Quality = &r.Quality.Items[i]
	}
	runs := c.RunIDs
	if c.ConversationID != "" {
		conv, err := v.App.IDs.Decode(string(models.PrefixConversation), c.ConversationID)
		if err == nil {
			out.Messages = v.messages(ctx, conv)
			if len(runs) == 0 {
				runs = v.conversationRuns(ctx, conv)
			}
		}
	}
	for _, id := range runs {
		if run, ok := v.aiRun(ctx, id); ok {
			out.Runs = append(out.Runs, run)
		}
	}
	out.Missing = len(out.Messages) == 0 && len(out.Runs) == 0
	return out
}

/*
messages reads a conversation's timeline.
*/
func (v *Viewer) messages(ctx context.Context, conv ids.UUID) []viewMessage {
	rows, err := v.App.DB.Query(ctx, `SELECT created_at, author_type, type, coalesce(body, '') FROM messages
		WHERE conversation_id = $1 ORDER BY created_at, id`, conv)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []viewMessage
	for rows.Next() {
		var m viewMessage
		if rows.Scan(&m.At, &m.Author, &m.Type, &m.Body) == nil && (m.Body != "" || m.Type != "event") {
			out = append(out, m)
		}
	}
	return out
}

/*
conversationRuns lists a conversation's AI runs, oldest first.
*/
func (v *Viewer) conversationRuns(ctx context.Context, conv ids.UUID) []string {
	rows, err := v.App.DB.Query(ctx, `SELECT id FROM ai_runs WHERE conversation_id = $1 ORDER BY created_at`, conv)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id ids.UUID
		if rows.Scan(&id) == nil {
			out = append(out, id.String())
		}
	}
	return out
}

/*
aiRun reads one AI run with its steps.
*/
func (v *Viewer) aiRun(ctx context.Context, id string) (viewAIRun, bool) {
	out := viewAIRun{ID: id}
	var outcome *string
	if v.App.DB.QueryRow(ctx, `SELECT status, outcome, cost_micros FROM ai_runs WHERE id = $1::uuid`, id).
		Scan(&out.Status, &outcome, &out.Cost) != nil {
		return out, false
	}
	if outcome != nil {
		out.Outcome = *outcome
	}
	rows, err := v.App.DB.Query(ctx, `SELECT seq, kind, coalesce(model, ''), status, input_tokens,
			cached_input_tokens, cache_write_tokens, output_tokens, cost_micros, coalesce(latency_ms, 0),
			coalesce(request::text, ''), coalesce(response::text, '')
		FROM ai_run_steps WHERE run_id = $1::uuid ORDER BY seq`, id)
	if err != nil {
		return out, true
	}
	defer rows.Close()
	for rows.Next() {
		var s viewStep
		if rows.Scan(&s.Seq, &s.Kind, &s.Model, &s.Status, &s.Fresh, &s.Cached, &s.Written, &s.Output, &s.Cost,
			&s.Latency, &s.Request, &s.Response) == nil {
			s.Request, s.Response = unquote(s.Request), unquote(s.Response)
			out.Steps = append(out.Steps, s)
		}
	}
	return out, true
}

/*
unquote reads a payload stored as a JSON string as its text.
*/
func unquote(s string) string {
	var text string
	if json.Unmarshal([]byte(s), &text) == nil {
		return text
	}
	return s
}

/*
write saves one page.
*/
func (v *Viewer) write(ctx context.Context, name, body string) error {
	return v.To.Put(ctx, name, []byte(body), filesystem.PutOptions{ContentType: "text/html"})
}

/*
jsonIndent writes a value as indented JSON.
*/
func jsonIndent(v any) (string, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	return string(out), err
}
