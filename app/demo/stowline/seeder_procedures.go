/*
|--------------------------------------------------------------------------
| Seeding The Procedures' History
|--------------------------------------------------------------------------
|
| A procedure the team has used for months has a past: versions
| published as it was improved, conversations that followed each one,
| the steps those conversations reached, the actions they called and
| the times a guardrail stepped in, and saved tests. Each of Stowline's
| procedures is published three times, first without its last step,
| then with its first step reworded, then as it stands, on days spread
| over the agent's time live. Every AI conversation on a procedure's
| topic followed the version live when it started. Most finish it and
| some stop part way, so the procedure's history and analytics read
| like a team's that uses it every day.
|
*/

package stowline

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
publish is one publish of a procedure: how long ago, and the note its
publisher left.
*/
type publish struct {
	ago  time.Duration
	note string
}

/*
procedurePublishes is when each version of a procedure went live,
oldest first.
*/
func procedurePublishes() []publish {
	return []publish{
		{82 * 24 * time.Hour, ""},
		{41 * 24 * time.Hour, "Reworded the first step after customers skipped it."},
		{12 * 24 * time.Hour, "Added the last step."},
	}
}

/*
followedProcedure is a published procedure the past runs follow: its
id, its name, and each version's steps and publish time, oldest first.
*/
type followedProcedure struct {
	id        ids.UUID
	name      string
	steps     [][]procedure.Step
	published []time.Time
}

/*
At returns the procedure as its version read, counted from zero: the
first lacks the last step and words the first step as first drafted,
the second rewords the first step, the third is the procedure as it
stands.
*/
func (p NamedProcedure) At(version int) procedure.Definition {
	def := p.Definition
	if version >= len(procedurePublishes())-1 || len(def.Steps) < 2 {
		return def
	}
	steps := append([]procedure.Step(nil), def.Steps[:len(def.Steps)-1]...)
	if version == 0 {
		steps[0].Text = p.FirstDraft
	}
	def.Steps = steps
	return def
}

/*
publishProcedures saves and publishes the team's procedures, each one
three times as the team improved it, dates the versions over the past
months and writes the saved tests.
*/
func (r *run) publishProcedures(ctx context.Context, published map[string]string) error {
	now := r.s.App.Clock.Now()
	r.followed = map[string]*followedProcedure{}
	for _, p := range Procedures(published) {
		f, err := r.publishProcedure(ctx, p, now)
		if err != nil {
			return err
		}
		r.followed[p.Topic] = f
	}
	return r.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := r.dateProcedures(ctx, tx, now); err != nil {
			return err
		}
		return r.procedureTests(ctx, tx, now)
	})
}

/*
publishProcedure creates one procedure and publishes each of its
versions in turn.
*/
func (r *run) publishProcedure(ctx context.Context, p NamedProcedure, now time.Time) (*followedProcedure, error) {
	a := r.s.App
	made, err := a.Actions.Procedures.Create(ctx, r.owner, p.Name)
	if err != nil {
		return nil, fmt.Errorf("create procedure %s: %w", p.Name, err)
	}
	f := &followedProcedure{id: made.ID, name: p.Name}
	lock := made.LockVersion
	for n, pub := range procedurePublishes() {
		at := p.At(n)
		def, _ := json.Marshal(at)
		saved, err := a.Actions.Procedures.Save(ctx, procedures.SaveInput{
			Membership: r.owner, ID: made.ID, LockVersion: lock, Definition: def,
		})
		if err != nil {
			return nil, fmt.Errorf("save procedure %s: %w", p.Name, err)
		}
		live, err := a.Actions.Procedures.PublishNote(ctx, r.owner, saved.ID, pub.note)
		if err != nil {
			return nil, fmt.Errorf("publish procedure %s: %w", p.Name, err)
		}
		lock = live.LockVersion
		f.steps = append(f.steps, procedure.Normalize(at).Steps)
		f.published = append(f.published, now.Add(-pub.ago))
	}
	return f, nil
}

/*
dateProcedures moves each version's publish time back to when the team
published it.
*/
func (r *run) dateProcedures(ctx context.Context, tx *database.Tx, now time.Time) error {
	publishes := procedurePublishes()
	for n, pub := range publishes {
		if _, err := tx.Exec(ctx, `UPDATE procedure_versions SET published_at = $3
			WHERE workspace_id = $1 AND version = $2`, r.ws.ID, n+1, now.Add(-pub.ago)); err != nil {
			return fmt.Errorf("date procedure versions: %w", err)
		}
	}
	last := publishes[len(publishes)-1]
	if _, err := tx.Exec(ctx, `UPDATE procedures SET published_at = $2 WHERE workspace_id = $1`,
		r.ws.ID, now.Add(-last.ago)); err != nil {
		return fmt.Errorf("date procedures: %w", err)
	}
	return nil
}

/*
SavedTest is a test the team keeps for a procedure: what the customer
writes, whether dimmi should hand the conversation over, and whether it
passed when last run.
*/
type SavedTest struct {
	Topic, Name, Message string
	HandsOver, Passed    bool
}

/*
SavedTests are the tests the team keeps for its procedures.
*/
func SavedTests() []SavedTest {
	parcel, refund, cancel := "missing_tracking", "void_refund", "cancel_account"
	return []SavedTest{
		{
			parcel, "Scan from two days ago", "My parcel has not moved since Tuesday, tracking 9400111899223817.",
			false, true,
		},
		{parcel, "No tracking number given", "Where is my package?", false, true},
		{
			parcel, "International, 12 days without a scan", "My parcel to Berlin has been stuck for 12 days.",
			false, false,
		},
		{
			refund, "Voided USPS label, asks when", "I voided a label last week. When do I get the money back?",
			false, true,
		},
		{
			refund, "Wants the refund on the card", "Can you put the voided label refund back on my card?",
			false, true,
		},
		{
			refund, "Top-up older than 120 days", "Refund my balance from the top-up I made in March to my card.",
			false, false,
		},
		{cancel, "Cancels over price", "I want to cancel, it is too expensive for us now.", true, true},
		{cancel, "Asks about the balance first", "If I cancel, what happens to my balance?", false, true},
	}
}

/*
procedureTests writes the procedures' saved tests with the result of
their last run, a few hours ago.
*/
func (r *run) procedureTests(ctx context.Context, tx *database.Tx, now time.Time) error {
	ran := now.Add(-3 * time.Hour)
	set := rowSet{name: "procedure_test_cases", cols: []string{
		"id", "workspace_id", "procedure_id", "name", "persona", "messages", "assertions", "last_result",
		"created_by_member_id", "created_at", "updated_at",
	}}
	for n, t := range SavedTests() {
		f, ok := r.followed[t.Topic]
		if !ok {
			continue
		}
		handed := map[bool]string{true: "handed over: yes", false: "handed over: no"}[t.HandsOver]
		if !t.Passed {
			handed = map[bool]string{true: "handed over: no", false: "handed over: yes"}[t.HandsOver]
		}
		made := now.Add(-time.Duration(20+n) * 24 * time.Hour)
		set.rows = append(set.rows, []any{
			stableID(made, "procedure-test/"+t.Name), r.ws.ID, f.id, t.Name,
			jsonOf(map[string]any{
				"name": "Ana Costa", "email": "ana.costa@example.test", "external_id": "test_ana_costa",
				"channel": "widget", "verified": true,
			}),
			jsonOf([]map[string]any{{"text": t.Message}}),
			jsonOf([]map[string]any{{"type": "procedure_matched"}, {"type": "handed_off", "handed_off": t.HandsOver}}),
			jsonOf(map[string]any{
				"passed": t.Passed, "ran_at": ran, "turns": []any{},
				"assertion_results": []map[string]any{
					{"type": "procedure_matched", "passed": true, "message": "dimmi followed this procedure."},
					{"type": "handed_off", "passed": t.Passed, "message": handed},
				},
			}),
			r.owner.Member.ID, made, ran,
		})
	}
	return insertMany(ctx, tx, set)
}

/*
publishedAction is what a past action call records about its action.
*/
type publishedAction struct {
	id, connection ids.UUID
	version        *ids.UUID
	sideEffect     string
	title          string
}

/*
publishedActions returns the workspace's actions by public id.
*/
func (r *run) publishedActions(ctx context.Context) (map[string]publishedAction, error) {
	out := map[string]publishedAction{}
	err := r.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, connection_id, published_version_id, side_effect, title
			FROM ai_actions WHERE workspace_id = $1`, r.ws.ID)
		if err != nil {
			return fmt.Errorf("list actions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var a publishedAction
			if err := rows.Scan(&a.id, &a.connection, &a.version, &a.sideEffect, &a.title); err != nil {
				return fmt.Errorf("read action: %w", err)
			}
			out[models.PublicID(models.PrefixAiAction, a.id)] = a
		}
		return rows.Err()
	})
	return out, err
}

/*
following is one conversation following a procedure: the version live
when it started, how many of its steps the conversation reached, the
guardrail events of its last run, and how many runs it has had so far.
*/
type following struct {
	proc    *followedProcedure
	version int
	reached int
	guards  []map[string]any
	runs    int
	called  map[string]bool
}

/*
follow returns the procedure a conversation follows, if its topic has
one and a version was live when it started. Two in three conversations
reach the last step; the rest stop part way. One in six trips a
guardrail.
*/
func (h *history) follow(c Conversation) *following {
	p, ok := h.run.followed[c.Topic]
	if !ok {
		return nil
	}
	version := 0
	for n, at := range p.published {
		if !c.CreatedAt.Before(at) {
			version = n + 1
		}
	}
	if version == 0 {
		return nil
	}
	f := &following{proc: p, version: version, called: map[string]bool{}}
	steps := p.steps[version-1]
	f.reached = len(steps)
	if h.rnd.IntN(3) == 0 {
		f.reached = 1 + h.rnd.IntN(len(steps))
	}
	f.guards = guardEvents(h.rnd.IntN(100))
	return f
}

/*
guardEvents are the guardrail events of a conversation for a draw out
of a hundred: one conversation in six trips one.
*/
func guardEvents(draw int) []map[string]any {
	event := func(kind string, attempt int) map[string]any {
		if attempt == 0 {
			return map[string]any{"type": kind}
		}
		return map[string]any{"type": kind, "attempt": attempt}
	}
	switch {
	case draw < 5:
		return []map[string]any{event("prerequisite_blocked", 1), event("compliance_retry", 2)}
	case draw < 11:
		return []map[string]any{event("compliance_retry", 1)}
	case draw < 15:
		return []map[string]any{event("awaiting_input", 0)}
	case draw < 17:
		return []map[string]any{event("compliance_failed_handoff", 0)}
	}
	return nil
}

/*
flags are the flags of a run that followed the procedure, none for a
run that followed none.
*/
func (f *following) flags() []string {
	if f == nil {
		return []string{}
	}
	return []string{"procedure:" + f.proc.id.String()}
}

/*
called is one action step a run finished: the conversation, the run and
when, what the conversation follows, and the step.
*/
type called struct {
	c    Conversation
	run  ids.UUID
	at   time.Time
	f    *following
	step procedure.Step
}

/*
trace is the procedure trace of a run, as the pipeline writes it, or
nothing for a run that followed no procedure. Each run of a
conversation gets one step further, the last as far as the conversation
got, and an action step that finishes records its call.
*/
func (h *history) trace(c Conversation, run ids.UUID, at time.Time, rp reply) any {
	f := rp.follow
	if f == nil {
		return nil
	}
	f.runs++
	final := rp.answer == lastAgentID(c)
	upto := min(f.reached, f.runs)
	if final {
		upto = f.reached
	}
	all := f.proc.steps[f.version-1]
	steps := make([]map[string]any, 0, upto)
	for n, st := range all[:upto] {
		if n == upto-1 && upto < len(all) {
			steps = append(steps, map[string]any{"step_id": st.ID, "kind": st.Kind, "status": procedure.StatusActive})
			continue
		}
		steps = append(steps, h.doneStep(called{c: c, run: run, at: at, f: f, step: st}))
	}
	status, guards := "active", []map[string]any{}
	if final {
		status, guards = "completed", append(guards, f.guards...)
	}
	if final && rp.handoff {
		status = "handed_off"
	}
	return jsonOf(map[string]any{
		"procedure_id": models.PublicID(models.PrefixProcedure, f.proc.id), "procedure_name": f.proc.name,
		"version": f.version, "status": status, "retries": retries(guards), "steps": steps, "guard_events": guards,
	})
}

/*
doneStep is a finished step of a trace. An action step names its
action and its call.
*/
func (h *history) doneStep(in called) map[string]any {
	step := map[string]any{"step_id": in.step.ID, "kind": in.step.Kind, "status": procedure.StatusDone}
	actions := in.step.Actions()
	if len(actions) == 0 {
		return step
	}
	step["action_id"] = actions[0].Ref
	if call := h.call(in, actions[0].Ref); call != "" {
		step["action_call_id"] = call
	}
	return step
}

/*
retries counts the replies the guardrails sent back.
*/
func retries(guards []map[string]any) int {
	n := 0
	for _, g := range guards {
		if g["type"] == "compliance_retry" {
			n++
		}
	}
	return n
}

/*
lastAgentID is the id of a conversation's last agent message.
*/
func lastAgentID(c Conversation) ids.UUID {
	for k, m := range slices.Backward(c.Messages) {
		if m.Author == "agent" {
			return stableID(m.At, fmt.Sprintf("%s/%d", c.ID, k))
		}
	}
	return ids.UUID{}
}

/*
call records the first call of a step's action in a conversation and
returns its public id, or nothing when it was already called. A write
fails more often than a read, and takes longer.
*/
func (h *history) call(in called, action string) string {
	c, run, at, f, st := in.c, in.run, in.at, in.f, in.step
	a, ok := h.actions[action]
	if !ok || f.called[st.ID] {
		return ""
	}
	f.called[st.ID] = true
	fails, slow := 4, 900
	if a.sideEffect == "write" {
		fails, slow = 14, 2400
	}
	state, code, status := "succeeded", any(nil), 200
	if h.rnd.IntN(100) < fails {
		state, code, status = "failed", "upstream_error", 502
	}
	id := stableID(at, fmt.Sprintf("%s/call/%s", run, st.ID))
	done := at.Add(time.Second)
	h.calls = append(h.calls, []any{
		id, h.run.ws.ID, a.id, a.version, a.connection, "live", a.sideEffect, state, code, status, run, c.ID,
		h.run.plan.Contacts[c.Contact].ID, a.title, 180 + h.rnd.IntN(slow), 1, at, at, done, at, done,
	})
	return models.PublicID(models.PrefixAiActionCall, id)
}

/*
callColumns are the columns a past action call is written with.
*/
func callColumns() []string {
	return []string{
		"id", "workspace_id", "action_id", "action_version_id", "connection_id", "mode", "side_effect", "state",
		"error_code", "http_status", "run_id", "conversation_id", "contact_id", "title", "latency_ms", "attempts",
		"requested_at", "sent_at", "completed_at", "created_at", "updated_at",
	}
}
