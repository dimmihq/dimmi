/*
|--------------------------------------------------------------------------
| Fake Decider
|--------------------------------------------------------------------------
|
| Tests script a decision model's answers without a network call or a
| cent spent, and read back every state and question it was asked. It
| only exists in the testing environment.
|
*/

package fake

import (
	"context"
	"errors"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
ErrNotTesting refuses the fake outside the testing environment.
*/
var ErrNotTesting = errors.New("decider fake: only in the testing environment")

/*
Step is one scripted call: the answers by question name, the usage,
or an error.
*/
type Step struct {
	Answers map[string]contracts.DecisionAnswer
	Usage   contracts.DecisionUsage
	Err     error
}

/*
Decider is a scripted contracts.Decider. Steps are served in order and
the last one repeats; a question no step answers gets a zero answer of
its type.
*/
type Decider struct {
	mu    sync.Mutex
	steps []Step
	sent  []contracts.DecisionRequest
}

/*
New returns the fake when testing is true, the environment's word.
*/
func New(testing bool) (*Decider, error) {
	if !testing {
		return nil, ErrNotTesting
	}
	return &Decider{}, nil
}

/*
Script sets the steps served in order.
*/
func (d *Decider) Script(steps ...Step) *Decider {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.steps = steps
	return d
}

/*
Sent returns every request the fake received.
*/
func (d *Decider) Sent() []contracts.DecisionRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]contracts.DecisionRequest{}, d.sent...)
}

/*
Decide serves the next step.
*/
func (d *Decider) Decide(_ context.Context, req contracts.DecisionRequest) (*contracts.DecisionResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sent = append(d.sent, req)

	var step Step
	if len(d.steps) > 0 {
		step = d.steps[0]
		if len(d.steps) > 1 {
			d.steps = d.steps[1:]
		}
	}
	if step.Err != nil {
		return nil, step.Err
	}

	out := &contracts.DecisionResponse{
		Model: "fake", Answers: map[string]contracts.DecisionAnswer{}, Usage: step.Usage,
	}
	for name, q := range req.Questions {
		answer, ok := step.Answers[name]
		if !ok {
			answer = contracts.DecisionAnswer{Type: q.Type}
		}
		out.Answers[name] = answer
	}
	return out, nil
}
