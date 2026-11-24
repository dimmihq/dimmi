/*
|--------------------------------------------------------------------------
| Routine Delivery
|--------------------------------------------------------------------------
|
| Once a routine's run ends, the people it reports to hear about it, as
| its notify policy says: only when it suggested something (and when it
| paused), every time, or only when it failed or paused. Teammates in
| the app get a notification, teammates on its email list the report
| with what it suggested, and its Slack destinations the report followed
| by one card per suggestion to apply or decline right there.
|
| Delivery is the server's, never the model's: it goes only where the
| admin pointed the routine, and only to teammates who may read what the
| routine reads. A dry run tells nobody. Each run is told once; what
| reached whom is kept on the run.
|
*/

package operatorroutines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/actions/operatorslack"
	"github.com/dimmihq/dimmi/app/ai/operator"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of what a delivery carries: the suggestions shown as cards
and the report's length in Slack.
*/
const (
	cardsPosted  = 10
	slackReport  = 2800
	reportLines  = 40
	reportLength = 12000
)

/*
Delivery tells a routine's people how its run went. Slack is nil when
the install has no Slack app.
*/
type Delivery struct {
	DB    *database.DB
	Clock clock.Clock
	Queue *queue.Queue
	Mail  *mail.Manager
	Slack *notifications.SlackChannel
	Links notifications.Links
}

/*
Result is what reached one target: the channel, who or where, and why
it did not, when it did not.
*/
type Result struct {
	Channel string `json:"channel"`
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

/*
told is everything a delivery reads about one run.
*/
type told struct {
	ws          models.Workspace
	routine     models.OperatorRoutine
	run         models.OperatorRoutineRun
	outcome     string
	report      string
	suggestions []models.OperatorChangeset
}

/*
Deliver tells the run's people, once.
*/
func (d *Delivery) Deliver(ctx context.Context, workspaceID, runID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())
	var t told
	var results []Result
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var ok bool
		var err error
		if t, ok, err = d.read(ctx, q, workspaceID, runID); err != nil || !ok {
			return err
		}
		results = append(results, d.inApp(ctx, tx, t)...)
		results = append(results, d.email(ctx, tx, t)...)
		return nil
	})
	if err != nil || t.run.ID == (ids.UUID{}) {
		return err
	}
	results = append(results, d.slack(ctx, t)...)
	return d.record(ctx, t.run, results)
}

/*
read loads the run and what it found, false when nobody is to be told:
a dry run, one told already, or a policy the outcome does not meet.
*/
func (d *Delivery) read(ctx context.Context, q *models.Queries, workspaceID, runID ids.UUID) (told, bool, error) {
	run, err := q.FindOperatorRoutineRunByID(ctx, models.FindOperatorRoutineRunByIDParams{
		WorkspaceID: workspaceID, ID: runID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return told{}, false, nil
	}
	if err != nil {
		return told{}, false, fmt.Errorf("find routine run: %w", err)
	}
	if run.Trigger == operator.TriggerDryRun || run.Notified {
		return told{}, false, nil
	}
	r, err := q.FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{
		WorkspaceID: workspaceID, ID: run.RoutineID,
	})
	if err != nil {
		return told{}, false, fmt.Errorf("find routine: %w", err)
	}
	t := told{routine: r, run: run, outcome: outcomeOf(run, r)}
	if !wanted(r.NotifyPolicy, t.outcome) {
		return told{}, false, nil
	}
	if t.ws, err = q.FindWorkspace(ctx, workspaceID); err != nil {
		return told{}, false, fmt.Errorf("find workspace: %w", err)
	}
	if t.report, err = report(ctx, q, run); err != nil {
		return told{}, false, err
	}
	t.suggestions, err = q.ListOperatorRunChangesets(ctx, models.ListOperatorRunChangesetsParams{
		WorkspaceID: workspaceID, RoutineRunID: &run.ID,
	})
	return t, err == nil, err
}

/*
outcomeOf is how the run ended for the people told: paused when it
paused the routine, else the run's own outcome.
*/
func outcomeOf(run models.OperatorRoutineRun, r models.OperatorRoutine) string {
	if r.Status == "paused" && r.PauseReason != nil {
		return "paused"
	}
	if run.Outcome == nil {
		return operator.RunNoop
	}
	return *run.Outcome
}

/*
wanted reports whether the policy tells about the outcome. A pause is
told whatever the policy, since the routine stopped until someone acts.
*/
func wanted(policy, outcome string) bool {
	switch {
	case outcome == "paused":
		return true
	case policy == "always":
		return outcome != operator.RunSkipped
	case policy == "on_error_only":
		return outcome == operator.RunFailed
	}
	return outcome == operator.RunFindings
}

/*
report is what Operator wrote at the end of the run: the thread's last
message, when a run wrote it.
*/
func report(ctx context.Context, q *models.Queries, run models.OperatorRoutineRun) (string, error) {
	if run.ThreadID == nil {
		return "", nil
	}
	msgs, err := q.ListOperatorMessages(ctx, models.ListOperatorMessagesParams{
		WorkspaceID: run.WorkspaceID, ThreadID: *run.ThreadID, RowLimit: 1,
	})
	if err != nil || len(msgs) == 0 || msgs[0].RunID == nil {
		return "", err
	}
	return msgs[0].Body, nil
}

/*
readers are the teammates among the list who may read what the routine
reads, with their memberships as they stand now.
*/
func readers(ctx context.Context, q *models.Queries, t told, list []ids.UUID) []models.WorkspaceMember {
	var out []models.WorkspaceMember
	for _, id := range slices.Compact(slices.Clone(list)) {
		m, ok, err := operator.LoadMember(ctx, q, t.ws.ID, id)
		if err == nil && ok && operator.RoutineReadable(m, t.routine) {
			out = append(out, m.Member)
		}
	}
	return out
}

/*
inApp queues the notification to the teammates the routine tells in the
app; the notifications keep their own preferences.
*/
func (d *Delivery) inApp(ctx context.Context, tx *database.Tx, t told) []Result {
	members := readers(ctx, models.New(tx), t, operator.DeliveryOf(t.routine.Delivery).InAppMemberIDs)
	if len(members) == 0 {
		return nil
	}
	targets := make([]ids.UUID, len(members))
	for i, m := range members {
		targets[i] = m.ID
	}
	err := d.Queue.PushTx(ctx, tx, notifications.Event{
		Trigger: notifications.TriggerRoutineFinished, WorkspaceID: t.ws.ID, Targets: targets,
		RoutineRunID: &t.run.ID, ActorType: "operator", OccurredAt: d.Clock.Now(),
	})
	out := make([]Result, len(members))
	for i, m := range members {
		out[i] = result("in_app", models.PublicID(models.PrefixMember, m.ID), err)
	}
	return out
}

/*
email queues the report to the teammates on the routine's email list.
*/
func (d *Delivery) email(ctx context.Context, tx *database.Tx, t told) []Result {
	q := models.New(tx)
	members := readers(ctx, q, t, operator.DeliveryOf(t.routine.Delivery).EmailMemberIDs)
	var out []Result
	for _, m := range members {
		user, err := q.FindUser(ctx, m.UserID)
		if err == nil {
			err = appmail.ToUser(d.Mail, user).QueueTx(ctx, tx, appmail.OperatorRoutineMail{
				Name: user.Name, Workspace: t.ws.Name, Routine: t.routine.Name, Outcome: t.outcome,
				Report: paragraphs(t.report), Suggestions: suggestions(t.suggestions),
				ReviewURL: d.Links.Operator(t.ws.Slug),
			})
		}
		out = append(out, result("email", models.PublicID(models.PrefixMember, m.ID), err))
	}
	return out
}

/*
slack posts the report, then a card per suggestion, to each of the
routine's Slack destinations.
*/
func (d *Delivery) slack(ctx context.Context, t told) []Result {
	list := operator.DeliveryOf(t.routine.Delivery).SlackDestinationIDs
	if len(list) == 0 {
		return nil
	}
	var out []Result
	for _, id := range list {
		target := models.PublicID(models.PrefixDestination, id)
		if d.Slack == nil {
			out = append(out, result("slack", target, errors.New("slack is not set up on this install")))
			continue
		}
		out = append(out, result("slack", target, d.post(ctx, t, id)))
	}
	return out
}

/*
post sends the run to one Slack destination.
*/
func (d *Delivery) post(ctx context.Context, t told, id ids.UUID) error {
	var row models.NotificationDestination
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindNotificationDestination(ctx, models.FindNotificationDestinationParams{
			WorkspaceID: t.ws.ID, ID: id,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("find destination: %w", err)
	}
	if row.Status != "active" {
		return errors.New("the destination is " + row.Status)
	}
	route := *notifications.RouteOf(row)
	if _, err := d.Slack.Post(ctx, route, d.summaryCard(t)); err != nil {
		return err
	}
	for i, c := range t.suggestions {
		if i == cardsPosted || c.Status != operator.StatusPending {
			continue
		}
		if _, err := d.Slack.Post(ctx, route, operatorslack.ChangesetCard(c, "")); err != nil {
			return err
		}
	}
	return nil
}

/*
summaryCard is the run's report as Slack shows it, with a link to the
routine in dimmi.
*/
func (d *Delivery) summaryCard(t told) slackdriver.Card {
	title := map[string]string{
		"findings": "has suggestions for you", "failed": "failed", "paused": "paused",
	}[t.outcome]
	if title == "" {
		title = "found nothing to change"
	}
	head := t.routine.Name + " " + title
	text := strings.TrimSpace(t.report)
	if text == "" {
		text = fallback(t)
	}
	runes := []rune(text)
	if len(runes) > slackReport {
		text = string(runes[:slackReport]) + "…"
	}
	return slackdriver.Card{
		Text: head, Lines: []string{"*" + slackdriver.Escape(head) + "*"}, Markdown: text,
		Buttons: []slackdriver.Button{{Label: "Open in dimmi", URL: d.Links.Operator(t.ws.Slug)}},
	}
}

/*
fallback says how the run ended when Operator wrote nothing.
*/
func fallback(t told) string {
	switch {
	case t.outcome == "paused" && t.routine.PauseReason != nil:
		return "The routine paused: " + strings.ReplaceAll(*t.routine.PauseReason, "_", " ") + "."
	case t.run.ErrorMessage != nil:
		return "The run failed: " + *t.run.ErrorMessage
	case t.run.SkipReason != nil:
		return "The run was skipped: " + strings.ReplaceAll(*t.run.SkipReason, "_", " ") + "."
	}
	return "Nothing needs attention."
}

/*
paragraphs splits the report into plain paragraphs for a mail.
*/
func paragraphs(report string) []string {
	if len(report) > reportLength {
		report = report[:reportLength]
	}
	var out []string
	for line := range strings.SplitSeq(report, "\n") {
		text := strings.TrimSpace(markdown.PlainText([]byte(line)))
		if text == "" {
			continue
		}
		out = append(out, text)
		if len(out) == reportLines {
			break
		}
	}
	return out
}

/*
suggestions writes the run's waiting changes for a mail.
*/
func suggestions(rows []models.OperatorChangeset) []appmail.RoutineSuggestion {
	var out []appmail.RoutineSuggestion
	for _, c := range rows {
		if c.Status != operator.StatusPending {
			continue
		}
		s := appmail.RoutineSuggestion{Title: c.Title}
		for _, op := range operator.Operations(c.Operations) {
			s.Operations = append(s.Operations, op.Label)
		}
		out = append(out, s)
	}
	return out
}

/*
result records what became of one target.
*/
func result(channel, target string, err error) Result {
	r := Result{Channel: channel, Target: target, OK: err == nil}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

/*
record keeps what reached whom on the run, and marks it told.
*/
func (d *Delivery) record(ctx context.Context, run models.OperatorRoutineRun, results []Result) error {
	encoded, err := json.Marshal(results)
	if err != nil || len(results) == 0 {
		encoded = json.RawMessage("[]")
	}
	return d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).MarkOperatorRoutineRunNotified(ctx, models.MarkOperatorRoutineRunNotifiedParams{
			WorkspaceID: run.WorkspaceID, ID: run.ID, DeliveryResults: encoded, Now: d.Clock.Now(),
		})
	})
}
