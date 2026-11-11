/*
|--------------------------------------------------------------------------
| Steps That Call Out
|--------------------------------------------------------------------------
|
| An AI path and a webhook leave the database, so they never run inside
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

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
call holds the traveler while the step's call is made.
*/
func (r *run) call(n Node) outcome {
	timeout := time.Duration(max(r.s.Config.AITimeoutSeconds, 1)) * time.Second
	if n.Kind == KindWebhook {
		timeout = time.Duration(max(r.s.Config.WebhookTimeoutSeconds, 1)) * time.Second
	}

	return outcome{wait: &wait{Kind: "call", Until: r.now.Add(3*timeout + time.Minute)}}
}

/*
called follows the call's answer, or the fallback once the time is up.
*/
func (r *run) called(n Node, w wait) outcome {
	if w.Result == "" && r.now.Before(w.Until) {
		return outcome{stay: true}
	}

	if n.Kind == KindWebhook {
		return outcome{path: PathNext, result: "webhook:" + cut(w.Result, 30)}
	}

	paths := pathIDs(settingsOf[AIPathSettings](n).Paths, func(p AIPath) string { return p.ID })
	if slices.Contains(paths, w.Result) {
		return outcome{path: w.Result, result: "ai:" + w.Result}
	}

	return outcome{path: PathFallback, result: "ai_fallback"}
}

/*
Call makes the call a waiting step needs and moves the journey on.
*/
func (s *Series) Call(ctx context.Context, a jobs.CallSeriesStepArgs) error {
	var r *run
	var n Node

	err := s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		var err error
		r, n, err = s.callee(ctx, tx, a)

		return err
	})
	if err != nil || r == nil {
		return gone(err, "read call")
	}

	result := s.answer(ctx, r, n)

	return s.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		w := r.waitOf()
		w.Result = result
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
answer makes the call and returns what the step follows.
*/
func (s *Series) answer(ctx context.Context, r *run, n Node) string {
	if n.Kind == KindWebhook {
		return s.postWebhook(ctx, r, n)
	}

	if s.Picker == nil {
		return PathFallback
	}

	settings := settingsOf[AIPathSettings](n)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()

	picked, err := s.Picker.Pick(ctx, PickInput{
		WorkspaceID: r.e.WorkspaceID, Prompt: settings.Prompt, Paths: settings.Paths, Profile: profileOf(r),
	})
	if err != nil || picked == "" {
		return PathFallback
	}

	return picked
}

/*
postWebhook posts the traveler and the step, and returns the status.
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

	resp, err := s.HTTP.New().
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
