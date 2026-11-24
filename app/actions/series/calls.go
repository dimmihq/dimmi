/*
|--------------------------------------------------------------------------
| Steps That Call Out
|--------------------------------------------------------------------------
|
| An AI path, an extract step, an action and a webhook leave the
| database, so they never run inside
| the lock that moves the journey. The step waits, a job of its own
| makes the call and stores the answer on the wait, and the journey
| moves on with it. An AI that cannot choose, fails or takes too long
| sends the traveler down the fallback path; a webhook that fails
| still moves them on, with its status in the trace. One call is made
| per visit: a retried move reuses the stored answer.
|
*/

package series

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
call holds the traveler while the step's call is made.
*/
func (r *run) call(n Node) outcome {
	timeout := time.Duration(max(r.s.Config.AITimeoutSeconds, 1)) * time.Second
	if n.Kind == KindWebhook || n.Kind == KindRunAction {
		timeout = time.Duration(max(r.s.Config.WebhookTimeoutSeconds, 1)) * time.Second
	}

	return outcome{wait: &wait{Kind: "call", Until: r.now.Add(3*timeout + time.Minute)}}
}

/*
called follows the call's answer, or the fallback once the time is up.
*/
func (r *run) called(ctx context.Context, n Node, w wait) (outcome, error) {
	if w.Result == "" && r.now.Before(w.Until) {
		return outcome{stay: true}, nil
	}

	switch n.Kind {
	case KindWebhook:
		return outcome{path: PathNext, result: "webhook:" + cut(w.Result, 30)}, nil
	case KindExtract:
		return r.extracted(ctx, n, w)
	case KindRunAction:
		return r.acted(ctx, n, w)
	}

	paths := pathIDs(settingsOf[AIPathSettings](n).Paths, func(p AIPath) string { return p.ID })
	if slices.Contains(paths, w.Result) {
		return outcome{path: w.Result, result: "ai:" + w.Result}, nil
	}

	return outcome{path: PathFallback, result: "ai_fallback"}, nil
}

/*
Call makes the call a waiting step needs and moves the journey on.
*/
func (s *Series) Call(ctx context.Context, a jobs.CallSeriesStepArgs) error {
	var c *stepCall

	err := s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		r, n, err := s.callee(ctx, tx, a)
		if err != nil || r == nil {
			return err
		}

		c, err = s.prepareCall(ctx, r, n)

		return err
	})
	if err != nil || c == nil {
		return gone(err, "read call")
	}

	result, values := s.answer(ctx, c)

	return s.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		w := c.r.waitOf()
		w.Result, w.Values = result, values
		raw, _ := json.Marshal(w)

		if err := q.SetSeriesEnrollmentWait(ctx, models.SetSeriesEnrollmentWaitParams{
			WorkspaceID: a.WorkspaceID, ID: a.EnrollmentID, Seq: a.Seq, Wait: raw, Now: s.Clock.Now(),
		}); err != nil {
			return wrap("store call answer", err)
		}

		return s.Queue.Push(ctx, jobs.AdvanceSeriesEnrollmentArgs(a))
	})
}

/*
callee reads the journey and step a call is for; nil when the journey
moved on meanwhile.
*/
func (s *Series) callee(ctx context.Context, tx *database.Tx, a jobs.CallSeriesStepArgs) (*run, Node, error) {
	q := models.New(tx)

	e, err := q.FindSeriesEnrollment(ctx, models.FindSeriesEnrollmentParams{
		WorkspaceID: a.WorkspaceID,
		ID:          a.EnrollmentID,
	})
	if err != nil || e.Seq != a.Seq || e.Status != "waiting" {
		return nil, Node{}, gone(err, "find enrollment")
	}

	row, err := find(ctx, q, e.WorkspaceID, e.SeriesID)
	if err != nil {
		return nil, Node{}, err
	}

	r, err := s.open(ctx, tx, row, e)
	if err != nil {
		return nil, Node{}, err
	}

	n, ok := r.g.node(e.NodeID)
	if !ok || r.waitOf().Kind != "call" || r.waitOf().Result != "" {
		return nil, Node{}, nil
	}

	return r, n, nil
}

/*
stepCall is what a call needs, read in the journey's transaction: the
journey, the step, and the person as the AI reads them.
*/
type stepCall struct {
	r       *run
	n       Node
	profile string
	action  *actionkit.AutomationCall
}

/*
prepareCall reads what the step's call needs.
*/
func (s *Series) prepareCall(ctx context.Context, r *run, n Node) (*stepCall, error) {
	c := &stepCall{r: r, n: n}

	var err error

	switch n.Kind {
	case KindAIPath, KindExtract:
		c.profile, err = profileWithEvents(ctx, r)
	case KindRunAction:
		c.action = s.prepareAction(ctx, r, n)
	}

	return c, err
}

/*
answer makes the call and returns what the step follows, with the
values it writes to the person.
*/
func (s *Series) answer(ctx context.Context, c *stepCall) (string, map[string]json.RawMessage) {
	r, n := c.r, c.n

	switch n.Kind {
	case KindWebhook:
		return s.postWebhook(ctx, r, n), nil
	case KindExtract:
		return s.extract(ctx, c)
	case KindRunAction:
		return s.runAction(ctx, c)
	}

	if s.Decisions == nil {
		return PathFallback, nil
	}

	settings := settingsOf[AIPathSettings](n)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()

	options := make([]aidecision.Option, 0, len(settings.Paths))
	for _, p := range settings.Paths {
		options = append(options, aidecision.Option{ID: p.ID, Label: p.Label, Description: p.Description})
	}

	picked, err := s.Decisions.Choose(ctx, aidecision.Choice{
		Subject:      aidecision.Subject{WorkspaceID: r.e.WorkspaceID, Context: c.profile},
		Instructions: settings.Prompt, Options: options,
	})
	if err != nil || picked.OptionID == "" {
		return PathFallback, nil
	}

	return picked.OptionID, nil
}

/*
postWebhook posts the traveler and the step and returns the status. A
redirect is never followed: its status is returned as it is.
*/
func (s *Series) postWebhook(ctx context.Context, r *run, n Node) string {
	if s.HTTP == nil {
		return "unavailable"
	}

	body := map[string]any{
		"type": "series.step", "series_id": models.PublicID(models.PrefixSeries, r.e.SeriesID),
		"enrollment_id": models.PublicID(models.PrefixSeriesEnrollment, r.e.ID), "step_id": n.ID,
		"contacts": contactsOf(r.people),
	}
	if r.company != nil {
		body["company_id"] = models.PublicID(models.PrefixCompany, *r.company)
	}

	resp, err := s.HTTP.New().WithoutRedirects().
		Timeout(time.Duration(max(s.Config.WebhookTimeoutSeconds, 1))*time.Second).
		WithHeader("Idempotency-Key", r.e.ID.String()+":"+r.waitOf().StepID).
		Post(ctx, settingsOf[WebhookSettings](n).URL, body)
	if err != nil {
		return "failed"
	}

	return fmt.Sprintf("%d", resp.Status)
}

/*
profileOf writes what the AI reads about the traveler: names, plans,
attributes and company, never secrets.
*/
func profileOf(r *run) string {
	var b strings.Builder

	for i, c := range r.people {
		if i == 10 {
			break
		}

		fmt.Fprintf(&b, "Person: %s\nSigned up: %s\nAttributes: %s\n", deref(c.Name),
			c.CreatedAt.Format("2006-01-02"), cut(string(c.CustomAttributes), 1500))
	}

	return b.String()
}

/*
contactsOf lists the people a webhook names.
*/
func contactsOf(people []models.Contact) []map[string]any {
	out := make([]map[string]any, 0, len(people))
	for _, c := range people {
		out = append(out, map[string]any{
			"id": models.PublicID(models.PrefixContact, c.ID), "email": deref(c.Email), "name": deref(c.Name),
		})
	}

	return out
}

/*
deref reads optional text.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
