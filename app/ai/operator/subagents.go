/*
|--------------------------------------------------------------------------
| Subagents
|--------------------------------------------------------------------------
|
| A routine's work can be more than one run can read: hundreds of
| articles, a month of conversations. The main agent splits such work
| across subagents that run at the same time. Each is the same model
| with the same skills and reading tools, starts from nothing but the
| task it was given, and reports back what it found. It suggests
| nothing: subagents work at the same time and cannot see each other,
| so two of them would suggest the same change. The main agent waits
| for all of them, makes every suggestion from their reports, then
| saves the routine's state and writes the run's message. A subagent
| cannot start subagents, touch the routine's state or end the run. Its model
| calls are a run of their own, so its cost is counted once, and its
| steps show in the run's timeline under its name.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The bounds of splitting a run: how many subagents one call starts, how
many a run starts in all, and how long one may work.
*/
const (
	toolSubagents       = "run_subagents"
	maxSubagentsPerCall = 4
	maxSubagentsPerRun  = 8
	subagentTimeout     = 6 * time.Minute
)

/*
subagent marks a session as a subagent's: its number in the run, the
few words that name its part, and the main agent's session.
*/
type subagent struct {
	n      int
	title  string
	parent *session
}

/*
subagentTask is one part of the work as the main agent hands it over.
*/
type subagentTask struct {
	Title string `json:"title"`
	Task  string `json:"task"`
}

/*
subagentsSchema is the schema of run_subagents.
*/
const subagentsSchema = `{"type":"object","required":["subagents"],"properties":{"subagents":{"type":"array",
"minItems":1,"maxItems":4,"description":"One entry per subagent. Their parts must not overlap.",
"items":{"type":"object","required":["title","task"],"properties":{
"title":{"type":"string","maxLength":80,"description":"A few words naming this part, like 'Articles 26 to ` +
	`50'. The team sees it."},
"task":{"type":"string","maxLength":4000,"description":"Everything this subagent must know, since it sees ` +
	`nothing of your run: exactly what to read (every conversation number or id, written out), what to ` +
	`look for, and the name of the skill to read first."}}}}}}`

/*
subagentsTool is the tool that splits a routine's work.
*/
func (e *Engine) subagentsTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: toolSubagents,
		Description: "Split a large piece of this routine's work across up to 4 subagents that run at the same " +
			"time. Each reads its part and reports back to you what it found, with the evidence; it suggests " +
			"nothing, so every suggestion is yours to make from the reports. This call returns once all have " +
			"reported. Use it when the work is more than you can read yourself in one run. A run may start 8 " +
			"subagents in all.",
		Schema: schema(subagentsSchema),
	}, handle: e.runSubagents, routine: true, mainOnly: true, alone: true}
}

/*
runSubagents starts the subagents, waits for every report, and hands
the main agent what each did.
*/
func (e *Engine) runSubagents(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Subagents []subagentTask `json:"subagents"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if err := checkSubagents(s, in.Subagents); err != nil {
		return result{}, err
	}
	e.addStep(ctx, s, Step{Tool: toolSubagents, Summary: "Started " + counted(len(in.Subagents)), OK: true})
	subs := make([]*session, 0, len(in.Subagents))
	for _, task := range in.Subagents {
		sub, err := e.openSubagent(ctx, s, task)
		if err != nil {
			return result{}, err
		}
		subs = append(subs, sub)
	}
	reports := make([]string, len(subs))
	var wg sync.WaitGroup
	for i, sub := range subs {
		wg.Go(func() { reports[i] = e.workSubagent(ctx, sub) })
	}
	wg.Wait()
	var b strings.Builder
	for i, sub := range subs {
		s.tainted = s.tainted || sub.tainted
		s.routine.helpersCost += sub.tally.cost
		fmt.Fprintf(&b, "Subagent %d (%s)\nIts report:\n%s\n\n", sub.sub.n, sub.sub.title, reports[i])
	}
	b.WriteString("No subagent suggested anything: they cannot. Suggest what their reports back, each change once.")
	summary := counted(len(subs)) + " reported"
	return result{text: b.String(), step: Step{Tool: toolSubagents, Summary: summary}}, nil
}

/*
counted says how many subagents, in words a step shows.
*/
func counted(n int) string {
	if n == 1 {
		return "1 subagent"
	}
	return fmt.Sprintf("%d subagents", n)
}

/*
checkSubagents refuses a split that is empty, too wide, or past what the
run may start.
*/
func checkSubagents(s *session, tasks []subagentTask) error {
	if len(tasks) == 0 || len(tasks) > maxSubagentsPerCall {
		return fmt.Errorf("%w: \"subagents\" must hold 1 to %d entries", errBadArguments, maxSubagentsPerCall)
	}
	if left := maxSubagentsPerRun - s.routine.spawned; len(tasks) > left {
		return fmt.Errorf("%w: this run may start %d more subagent(s); do the rest yourself or leave it to the "+
			"next run", errBadArguments, left)
	}
	for i, task := range tasks {
		if strings.TrimSpace(task.Title) == "" || strings.TrimSpace(task.Task) == "" {
			return fmt.Errorf("%w: subagent %d needs a \"title\" and a \"task\"", errBadArguments, i+1)
		}
	}
	return nil
}

/*
openSubagent makes a subagent's session from the main agent's: the same
teammate, model, settings and routine, with nothing of the main run's
conversation, and an AI run of its own.
*/
func (e *Engine) openSubagent(ctx context.Context, s *session, task subagentTask) (*session, error) {
	s.routine.spawned++
	sub := *s
	sub.sub = &subagent{n: s.routine.spawned, title: truncate(strings.TrimSpace(task.Title), 80), parent: s}
	sub.question, sub.history, sub.memories = strings.TrimSpace(task.Task), nil, nil
	sub.steps, sub.proposed, sub.tally, sub.toolCalls = nil, nil, tally{}, 0
	sub.closing, sub.failure, sub.layer = false, nil, nil
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		sub.run, err = e.startRun(ctx, models.New(tx), &sub)
		return err
	})
	if err != nil {
		return nil, err
	}
	e.addStep(ctx, &sub, Step{Tool: "subagent", Summary: sub.sub.title, OK: true})
	return &sub, nil
}

/*
workSubagent runs a subagent to its report, closes its AI run and adds
its cost to the month. A subagent that did not finish says so, so the
main agent never reports its part as done.
*/
func (e *Engine) workSubagent(ctx context.Context, sub *session) string {
	runCtx, cancel := context.WithTimeout(ctx, subagentTimeout)
	text, status := e.converse(runCtx, sub)
	cancel()
	_ = e.DB.Transaction(context.WithoutCancel(ctx), func(ctx context.Context, tx *database.Tx) error {
		if err := e.finishRun(ctx, models.New(tx), sub, status); err != nil {
			return err
		}
		return addSpend(ctx, tx, e, sub)
	})
	done := status == "done" && strings.TrimSpace(text) != ""
	if !done {
		text = "This subagent did not finish its part and wrote no report. Treat its part as not done."
	}
	report := Step{Tool: "subagent_report", Summary: truncate(text, noteLength), OK: done}
	e.addStep(context.WithoutCancel(ctx), sub, report)
	return text
}

/*
addStep adds a step to the run's timeline, saves it and shows it live.
A subagent's step joins the main run's timeline under its number; the
main agent waits while subagents work, so only they write to it then.
*/
func (e *Engine) addStep(ctx context.Context, s *session, st Step) {
	if s.sub == nil {
		s.steps = append(s.steps, st)
		e.keepSteps(ctx, s)
		e.publishStep(ctx, s, st)
		return
	}
	st.Subagent = s.sub.n
	main := s.sub.parent
	main.routine.mu.Lock()
	main.steps = append(main.steps, st)
	e.keepSteps(ctx, main)
	main.routine.mu.Unlock()
	e.publishStep(ctx, main, st)
}
