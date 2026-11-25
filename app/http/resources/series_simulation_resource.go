/*
|--------------------------------------------------------------------------
| Series Simulation Resources
|--------------------------------------------------------------------------
|
| A series' dry simulation of one person as the API reads it.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/series"

/*
SeriesSimulation is a series' dry simulation of one person: whether
they would enter and why not, every step with the path it took, the
messages that would be sent, the changes that would be made, whether
the goal was reached, how the journey would end, and what a
scenario's person did each time the series asked.
*/
type SeriesSimulation struct {
	Object      string                     `json:"object"`
	Entered     bool                       `json:"entered"`
	Reason      string                     `json:"reason,omitempty"`
	Steps       []SeriesSimulationStep     `json:"steps"`
	Messages    []SeriesSimulationMessage  `json:"messages"`
	Actions     []SeriesSimulationAction   `json:"actions"`
	GoalReached bool                       `json:"goal_reached"`
	End         string                     `json:"end"`
	Decisions   []SeriesSimulationDecision `json:"decisions"`
}

/*
SeriesSimulationDecision is what a scenario's person did when the
series asked: before which step (an index into steps), in words, and
whether they did it.
*/
type SeriesSimulationDecision struct {
	Before int    `json:"before"`
	What   string `json:"what"`
	Did    bool   `json:"did"`
}

/*
SeriesSimulationStep is one step a dry simulation took; Skipped marks
a wait it did not hold the person on.
*/
type SeriesSimulationStep struct {
	NodeID  string         `json:"node_id"`
	Kind    string         `json:"kind"`
	Path    string         `json:"path,omitempty"`
	Result  string         `json:"result"`
	Skipped bool           `json:"skipped,omitempty"`
	Detail  map[string]any `json:"detail,omitempty"`
}

/*
SeriesSimulationMessage is a message that would be sent.
*/
type SeriesSimulationMessage struct {
	NodeID  string `json:"node_id"`
	Kind    string `json:"kind"`
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body"`
}

/*
SeriesSimulationAction is a change that would be made.
*/
type SeriesSimulationAction struct {
	NodeID string         `json:"node_id"`
	Kind   string         `json:"kind"`
	Detail map[string]any `json:"detail,omitempty"`
}

/*
NewSeriesSimulation shapes a dry simulation.
*/
func NewSeriesSimulation(t *series.SimTrace) SeriesSimulation {
	out := SeriesSimulation{
		Object: "series_simulation", Entered: t.Entered, Reason: t.Reason, GoalReached: t.Goal, End: t.End,
		Steps: make([]SeriesSimulationStep, len(t.Steps)), Messages: make([]SeriesSimulationMessage, len(t.Messages)),
		Actions:   make([]SeriesSimulationAction, len(t.Actions)),
		Decisions: make([]SeriesSimulationDecision, len(t.Decisions)),
	}
	for i, d := range t.Decisions {
		out.Decisions[i] = SeriesSimulationDecision(d)
	}
	for i, s := range t.Steps {
		out.Steps[i] = SeriesSimulationStep(s)
	}
	for i, m := range t.Messages {
		out.Messages[i] = SeriesSimulationMessage(m)
	}
	for i, a := range t.Actions {
		out.Actions[i] = SeriesSimulationAction(a)
	}
	return out
}
