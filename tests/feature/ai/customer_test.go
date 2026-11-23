package ai_test

import (
	"context"
	"sync"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

/*
scriptedCustomer plays a saved simulation's customer in tests: it sends
the saved message for each turn, by how many replies dimmi has given,
and is done when they run out. Moves, when set, are served instead, in
order, and fail makes it give no answer.
*/
type scriptedCustomer struct {
	mu       sync.Mutex
	moves    []pipeline.CustomerMove
	fail     error
	asked    []pipeline.CustomerTurn
	scenario string
	profiled []pipeline.CustomerProfile
}

/*
DescribeCustomer writes the scenario the test set, and keeps what it
was asked to write one from.
*/
func (c *scriptedCustomer) DescribeCustomer(_ context.Context, in pipeline.CustomerProfile) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.profiled = append(c.profiled, in)

	return c.scenario, nil
}

func (c *scriptedCustomer) PlayCustomer(_ context.Context, in pipeline.CustomerTurn) (pipeline.CustomerMove, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, in)
	if c.fail != nil {
		return pipeline.CustomerMove{}, c.fail
	}
	if len(c.moves) > 0 {
		move := c.moves[0]
		c.moves = c.moves[1:]
		return move, nil
	}
	at := 0
	for _, m := range in.Run {
		if m.From == "agent" {
			at++
		}
	}
	if at >= len(in.Saved) {
		return pipeline.CustomerMove{Done: true}, nil
	}
	next := in.Saved[at]

	return pipeline.CustomerMove{Message: next.Text, Confirm: next.Confirm, Saved: at + 1}, nil
}
