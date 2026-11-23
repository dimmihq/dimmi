/*
|--------------------------------------------------------------------------
| Operator Routine Requests
|--------------------------------------------------------------------------
|
| What the API accepts for routines: a routine to create or change, the
| routine and run a request names, and a run to start now.
|
*/

package requests

import (
	"time"
)

/*
OperatorRoutineBody is a routine's settings. On create, fields left out
come from the template named, or the custom one; on update, fields left
out keep their values.
*/
type OperatorRoutineBody struct {
	Name           *string                    `json:"name,omitempty"                  validate:"omitempty,max=120"`
	Description    *string                    `json:"description,omitempty"           validate:"omitempty,max=500"`
	Prompt         *string                    `json:"prompt,omitempty"                validate:"max=8000"`
	Cadence        *string                    `json:"cadence,omitempty"               validate:"omitempty,max=20"`
	CronExpression *string                    `json:"cron_expression,omitempty"       validate:"omitempty,max=100"`
	RunAt          *time.Time                 `json:"run_at,omitempty"`
	Timezone       *string                    `json:"timezone,omitempty"              validate:"omitempty,max=64"`
	OwnerID        *string                    `json:"owner_id,omitempty"              validate:"omitempty,max=64"`
	InboxIDs       []string                   `json:"inbox_ids,omitempty"             validate:"omitempty,max=100"`
	Scope          *OperatorRoutineScopeInput `json:"scope,omitempty"`
	NotifyPolicy   *string                    `json:"notify_policy,omitempty"         validate:"omitempty,max=20"`

	Delivery *OperatorRoutineDeliveryInput `json:"delivery,omitempty"`
}

/*
StoreOperatorRoutineRequest is the input of POST /operator/routines.
*/
type StoreOperatorRoutineRequest struct {
	Body struct {
		OperatorRoutineBody
		TemplateKey *string `json:"template_key,omitempty" validate:"omitempty,max=64"`
	}
}

/*
UpdateOperatorRoutineRequest is the input of PATCH
/operator/routines/{routine}: settings to change, active or paused, and
reset_state to forget what the routine kept between runs.
*/
type UpdateOperatorRoutineRequest struct {
	Routine string `path:"routine"`
	Body    struct {
		OperatorRoutineBody
		Status     *string `json:"status,omitempty"       validate:"omitempty,max=20"`
		ResetState bool    `json:"reset_state,omitempty"`
	}
}

/*
OperatorRoutineRequest names one routine.
*/
type OperatorRoutineRequest struct {
	Routine string `path:"routine"`
}
