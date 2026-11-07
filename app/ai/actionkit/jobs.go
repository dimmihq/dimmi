/*
|--------------------------------------------------------------------------
| Call Jobs
|--------------------------------------------------------------------------
|
| The work a call leaves for later. An approved call executes in a job,
| unique per call, so however many approvals race it runs once; a call
| that ended wakes the agent in a second job, so a slow model never
| makes the request run again; a call waiting for a decision expires in
| a third, scheduled at its deadline. The workers live with the agent's
| jobs; the payloads live here, where the calls are pushed.
|
*/

package actionkit

import (
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ExecuteCallArgs runs one approved call.
*/
type ExecuteCallArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	CallID      ids.UUID `json:"call_id"`
}

/*
Kind names the job.
*/
func (ExecuteCallArgs) Kind() string { return "execute_ai_action_call" }

/*
Options keep one execution per call, ever, and try twice at most: the
second attempt only finds out whether the first one sent anything.
*/
func (ExecuteCallArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 2, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
FollowUpArgs wakes the agent to tell the customer how a call ended.
*/
type FollowUpArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	CallID      ids.UUID `json:"call_id"`
}

/*
Kind names the job.
*/
func (FollowUpArgs) Kind() string { return "follow_up_ai_action_call" }

/*
Options keep one follow-up per call.
*/
func (FollowUpArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
ExpireCallArgs ends a call whose deadline passed while it waited. At is
the deadline the job was scheduled for, in Unix milliseconds: a call
whose deadline moved since, because the customer confirmed it and it
now waits for a teammate, has a job of its own.
*/
type ExpireCallArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	CallID      ids.UUID `json:"call_id"`
	At          int64    `json:"at"`
}

/*
Kind names the job.
*/
func (ExpireCallArgs) Kind() string { return "expire_ai_action_call" }

/*
Options keep one expiry per call and deadline.
*/
func (ExpireCallArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByArgs: true}}
}
