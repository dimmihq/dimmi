/*
|--------------------------------------------------------------------------
| Workflow Goals
|--------------------------------------------------------------------------
|
| A goal says what a run that worked looks like, so analytics can count
| more than runs that reached their end. The conversation is resolved
| without going to a person, or the customer rates it at least a score,
| within a window counted from when the run started. A goal never
| changes what a run does.
|
*/

package workflowspec

import (
	"time"
)

/*
The goal kinds.
*/
const (
	GoalResolved = "resolved"
	GoalCSAT     = "csat"
)

/*
Goal is a workflow's goal: Kind is resolved or csat, Window how long
after the run starts it may be reached ("7d"), and MinRating the lowest
rating, 1 to 5, a csat goal accepts.
*/
type Goal struct {
	Kind      string `json:"kind"`
	Window    string `json:"window"`
	MinRating int    `json:"min_rating,omitempty"`
}

/*
DefaultGoalWindow is the window of a goal that names none.
*/
const DefaultGoalWindow = 7 * 24 * time.Hour

/*
WindowOf returns how long the goal may take, the default for none.
*/
func (g Goal) WindowOf() time.Duration {
	d, err := ParseDuration(g.Window)
	if err != nil || d <= 0 {
		return DefaultGoalWindow
	}

	return d
}

/*
checkGoal checks the goal's kind, window and rating.
*/
func (c *checker) checkGoal() {
	g := c.spec.Goal
	if g == nil {
		return
	}

	switch g.Kind {
	case GoalResolved:
	case GoalCSAT:
		if g.MinRating < 1 || g.MinRating > 5 {
			c.fail(Issue{Code: CodeInvalidSetting, Field: "goal.min_rating", Message: "Use a rating from 1 to 5."})
		}
	default:
		c.fail(Issue{Code: CodeInvalidSetting, Field: "goal.kind", Message: "Use resolved or csat."})
	}

	if g.Window == "" {
		return
	}

	d, err := ParseDuration(g.Window)
	if err != nil || d < time.Hour || d > 90*24*time.Hour {
		c.fail(Issue{Code: CodeInvalidDuration, Field: "goal.window", Message: "Use 1 hour to 90 days."})
	}
}
