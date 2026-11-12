/*
|--------------------------------------------------------------------------
| Step Kinds
|--------------------------------------------------------------------------
|
| Every step kind is registered once: whether it speaks to the customer,
| which fixed paths it may have, whether people add their own paths to
| it and how many. A step that speaks to the customer makes the whole
| workflow customer-facing, which decides how it competes with other
| workflows for a conversation.
|
| A kind the engine cannot run yet is registered but not ready: a
| definition may hold it as a draft, and publishing it is refused.
|
*/

package workflowspec

import "slices"

/*
The step kinds.
*/
const (
	KindMessage       = "message"
	KindButtons       = "buttons"
	KindCollect       = "collect"
	KindReplyTime     = "reply_time"
	KindDimmi         = "dimmi"
	KindBranch        = "branch"
	KindClassify      = "classify"
	KindWait          = "wait"
	KindRunWorkflow   = "run_workflow"
	KindAssign        = "assign"
	KindUnassign      = "unassign"
	KindTag           = "tag"
	KindUntag         = "untag"
	KindPriority      = "priority"
	KindNote          = "note"
	KindUpdateContact = "update_contact"
	KindSnooze        = "snooze"
	KindClose         = "close"
	KindApplySLA      = "apply_sla"
	KindHTTPRequest   = "http_request"
)

/*
The fixed path outcomes.
*/
const (
	OutcomeTyped       = "typed"
	OutcomeTimeout     = "timeout"
	OutcomeInvalid     = "invalid"
	OutcomeResolved    = "resolved"
	OutcomeHandedOver  = "handed_over"
	OutcomeQuiet       = "quiet"
	OutcomeElse        = "else"
	OutcomeNone        = "none"
	OutcomeInterrupted = "interrupted"
	OutcomeFailure     = "failure"
)

/*
Kind describes one step kind. Fixed lists the outcomes of its fixed
paths; MinPaths and MaxPaths bound the paths people add, zero meaning
none. Required names a fixed path every such step has.
*/
type Kind struct {
	Key            string
	CustomerFacing bool
	Fixed          []string
	Required       []string
	MinPaths       int
	MaxPaths       int
	Ready          bool
	Action         bool
}

/*
Kinds returns every step kind.
*/
func Kinds() []Kind {
	return []Kind{
		{Key: KindMessage, CustomerFacing: true, Ready: true},
		{
			Key: KindButtons, CustomerFacing: true, Fixed: []string{OutcomeTyped, OutcomeTimeout},
			MinPaths: 2, MaxPaths: 6, Ready: true,
		},
		{Key: KindCollect, CustomerFacing: true, Fixed: []string{OutcomeInvalid, OutcomeTimeout}, Ready: true},
		{Key: KindReplyTime, CustomerFacing: true, Ready: true},
		{
			Key: KindDimmi, CustomerFacing: true, Fixed: []string{OutcomeResolved, OutcomeHandedOver, OutcomeQuiet},
			Ready: true,
		},
		{
			Key: KindBranch, Fixed: []string{OutcomeElse}, Required: []string{OutcomeElse},
			MinPaths: 1, MaxPaths: 10, Ready: true,
		},
		{
			Key: KindClassify, Fixed: []string{OutcomeNone}, Required: []string{OutcomeNone}, MinPaths: 2, MaxPaths: 10,
			Ready: true,
		},
		{Key: KindWait, Fixed: []string{OutcomeInterrupted}, Ready: true},
		{Key: KindRunWorkflow, Ready: true},
		{Key: KindAssign, Ready: true, Action: true},
		{Key: KindUnassign, Ready: true, Action: true},
		{Key: KindTag, Ready: true, Action: true},
		{Key: KindUntag, Ready: true, Action: true},
		{Key: KindPriority, Ready: true, Action: true},
		{Key: KindNote, Ready: true, Action: true},
		{Key: KindUpdateContact, Ready: true, Action: true},
		{Key: KindSnooze, Ready: true, Action: true},
		{Key: KindClose, Ready: true, Action: true},
		{Key: KindApplySLA, Action: true},
		{Key: KindHTTPRequest, Fixed: []string{OutcomeFailure}, Ready: true, Action: true},
	}
}

/*
FindKind returns the step kind with the key.
*/
func FindKind(key string) (Kind, bool) {
	kinds := Kinds()
	i := slices.IndexFunc(kinds, func(k Kind) bool { return k.Key == key })
	if i < 0 {
		return Kind{}, false
	}

	return kinds[i], true
}

/*
userPaths returns the paths of a step that are not fixed outcomes.
*/
func userPaths(s Step) []Path {
	var out []Path
	for _, p := range s.Paths {
		if p.Outcome == "" {
			out = append(out, p)
		}
	}

	return out
}

/*
PathFor returns the step's path with the outcome, if it has one.
*/
func PathFor(s Step, outcome string) (Path, bool) {
	for _, p := range s.Paths {
		if p.Outcome == outcome {
			return p, true
		}
	}

	return Path{}, false
}
