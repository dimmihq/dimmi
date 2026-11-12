/*
|--------------------------------------------------------------------------
| Definition Issues
|--------------------------------------------------------------------------
|
| Checking a definition lists every issue at once, each with a code and
| the step, path and setting it is about, so the builder can mark every
| card that needs attention. Errors stop a publish. Warnings do not, but
| the publisher has to acknowledge them, because each one describes a
| conversation that will likely end up somewhere nobody intended.
|
*/

package workflowspec

import (
	"fmt"
	"strings"
)

/*
The error codes.
*/
const (
	CodeSchema                    = "schema"
	CodeOpenerShape               = "opener_shape"
	CodeStepUnavailableForTrigger = "step_unavailable_for_trigger"
	CodeStepNotAvailable          = "step_not_available"
	CodeFieldUnavailableTrigger   = "field_unavailable_for_trigger"
	CodeUnknownVariable           = "unknown_variable"
	CodeMissingSetting            = "missing_setting"
	CodeInvalidSetting            = "invalid_setting"
	CodeReferenceMissing          = "reference_missing"
	CodeCallFromBackground        = "customer_facing_call_from_background"
	CodeSelfCall                  = "self_call"
	CodeCallCycle                 = "call_cycle"
	CodeLimitExceeded             = "limit_exceeded"
	CodeDuplicateID               = "duplicate_id"
	CodeInvalidDuration           = "invalid_duration"
	CodeChannelUnsupported        = "channel_unsupported"
	CodeInvalidCondition          = "invalid_condition"
)

/*
The warning codes.
*/
const (
	WarnDimmiNotReady      = "dimmi_not_ready"
	WarnPathEndsUnowned    = "path_ends_unowned"
	WarnHandoverSilent     = "handover_notice_silent"
	WarnHTTPRawURL         = "http_raw_url"
	WarnClaimedField       = "claimed_field"
	WarnIgnoredOnReusable  = "ignored_on_reusable"
	WarnReferenceArchived  = "reference_archived"
	WarnElseEmptyOnRouting = "else_empty_on_audience_branch"
)

/*
Issue is one error or warning about a definition.
*/
type Issue struct {
	Code    string `json:"code"`
	StepID  string `json:"step_id,omitempty"`
	PathID  string `json:"path_id,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

/*
Report holds what checking a definition found.
*/
type Report struct {
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

/*
OK reports whether the definition has no errors.
*/
func (r Report) OK() bool {
	return len(r.Errors) == 0
}

/*
Err returns the errors as an error, or nil when there are none.
*/
func (r Report) Err() error {
	if r.OK() {
		return nil
	}

	return &InvalidError{Errors: r.Errors}
}

/*
InvalidError carries the errors that stop a definition from running.
*/
type InvalidError struct {
	Errors []Issue
}

/*
Error lists the error codes in one line.
*/
func (e *InvalidError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, issue := range e.Errors {
		where := issue.StepID
		if where == "" {
			where = issue.Field
		}

		parts = append(parts, strings.TrimSuffix(fmt.Sprintf("%s %s", issue.Code, where), " "))
	}

	return "workflow invalid: " + strings.Join(parts, ", ")
}
