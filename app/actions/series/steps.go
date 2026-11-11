/*
|--------------------------------------------------------------------------
| Steps
|--------------------------------------------------------------------------
|
| Each kind of step decides on arrival where the traveler goes: down a
| path at once, or onto a wait that is asked again later. Checks look at
| the traveler as they are now. A split sends each traveler down one
| variant for good, by a fixed hash of the journey, so a retry or a
| second look always gives the same answer.
|
*/

package series

import (
	"context"
	"hash/fnv"
)

/*
enter runs a step the traveler just reached.
*/
func (r *run) enter(ctx context.Context, n Node, stepID string) (outcome, error) {
	switch {
	case n.Kind == KindEntry:
		return outcome{path: PathNext}, nil
	case n.Kind == KindExit:
		return outcome{end: "finished", result: "exit_step"}, nil
	case IsMessage(n.Kind):
		return r.message(ctx, n, stepID)
	case n.Kind == KindWait:
		return r.waitStep(ctx, n)
	case n.Kind == KindWaitFor:
		return r.waitFor(ctx, n, wait{}, false)
	case n.Kind == KindCondition:
		return r.condition(ctx, n)
	case n.Kind == KindSplit:
		return r.split(n), nil
	case n.Kind == KindAIPath || n.Kind == KindWebhook:
		return r.call(n), nil
	}

	return r.action(ctx, n)
}

/*
resume asks a waiting step again; stay keeps the traveler waiting.
*/
func (r *run) resume(ctx context.Context, n Node, w wait) (outcome, error) {
	due := !w.Until.IsZero() && !r.now.Before(w.Until)

	switch w.Kind {
	case "timer":
		if !due {
			return outcome{stay: true}, nil
		}

		return outcome{path: PathNext, result: "waited"}, nil
	case "cooloff":
		if !due {
			return outcome{stay: true}, nil
		}

		return r.message(ctx, n, w.StepID)
	case "send":
		return r.delivered(ctx, n, w)
	case "match":
		return r.waitFor(ctx, n, w, due)
	case "call":
		return r.called(n, w), nil
	}

	return outcome{path: PathNext}, nil
}

/*
condition takes the first path whose test matches, else everyone else.
*/
func (r *run) condition(ctx context.Context, n Node) (outcome, error) {
	for _, p := range settingsOf[ConditionSettings](n).Paths {
		ok, err := r.test(ctx, p.Rules, "", p.Engagement)
		if err != nil {
			return outcome{}, err
		}

		if ok {
			return outcome{path: p.ID, result: "matched"}, nil
		}
	}

	return outcome{path: PathElse, result: "else"}, nil
}

/*
waitFor takes the matched path once the test passes, the timed-out
path when the time is up, and waits otherwise.
*/
func (r *run) waitFor(ctx context.Context, n Node, w wait, due bool) (outcome, error) {
	s := settingsOf[WaitForSettings](n)

	ok, err := r.test(ctx, s.Rules, s.Event, s.Engagement)
	if err != nil {
		return outcome{}, err
	}

	switch {
	case ok:
		return outcome{path: PathMatched, result: "matched"}, nil
	case due:
		return outcome{path: PathTimedOut, result: "timed_out"}, nil
	case w.Kind == "match":
		return outcome{stay: true}, nil
	}

	d, _ := durationOf(s.Amount, s.Unit)

	return outcome{wait: &wait{Kind: "match", Until: r.now.Add(d)}}, nil
}

/*
test reports whether the traveler matches the rules, did the event
since entering, and acted on the message, whichever are given.
*/
func (r *run) test(ctx context.Context, rules []byte, event string, e *Engagement) (bool, error) {
	if len(rules) > 0 {
		ok, err := r.matches(ctx, nil, rules)
		if err != nil || !ok {
			return false, err
		}
	}

	if event != "" {
		ok, err := r.didEvent(ctx, event, r.e.EnteredAt)
		if err != nil || !ok {
			return false, err
		}
	}

	if e == nil {
		return len(rules) > 0 || event != "", nil
	}

	return r.engaged(ctx, *e)
}

/*
split sends the traveler down their variant: the winner once there is
one, else the variant they were given, else one picked by weight.
*/
func (r *run) split(n Node) outcome {
	s := settingsOf[SplitSettings](n)
	if s.Winner != "" {
		return outcome{path: s.Winner, result: "winner"}
	}

	if got, ok := r.variants[n.ID]; ok {
		return outcome{path: got, result: "variant"}
	}

	total := 0
	for _, v := range s.Variants {
		total += max(v.Weight, 0)
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(r.e.ID.String() + ":" + n.ID))
	pick := int(h.Sum32() % uint32(max(total, 1)))

	choice := ""
	for _, v := range s.Variants {
		if pick < v.Weight {
			choice = v.ID

			break
		}

		pick -= max(v.Weight, 0)
	}

	r.variants[n.ID] = choice

	return outcome{path: choice, result: "variant"}
}
