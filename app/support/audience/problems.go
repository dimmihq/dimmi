/*
|--------------------------------------------------------------------------
| Rule Problems
|--------------------------------------------------------------------------
|
| A rule that cannot be trusted is refused with every problem at once,
| each naming its code and the JSON path of the condition, so the
| builder marks every bad leaf in one pass instead of one per save.
|
*/

package audience

import (
	"fmt"
	"strings"
)

/*
The problem codes a rule can be refused with.
*/
const (
	CodeSchema               = "schema"
	CodeLimitExceeded        = "limit_exceeded"
	CodeUnknownAttribute     = "unknown_attribute"
	CodeOpInvalidForType     = "op_invalid_for_type"
	CodeValueInvalid         = "value_invalid"
	CodeFieldUnavailable     = "field_unavailable_for_surface"
	CodeClaimedOnAccessGate  = "claimed_field_on_access_gate"
	CodeAudienceCycle        = "audience_cycle"
	CodeMissingReference     = "missing_reference"
	CodeFieldUnavailableHere = "field_unavailable_for_trigger"
)

/*
Problem is one reason a rule is refused: a code, the JSON path of the
node, the field it reads when it is a leaf, and a sentence for people.
*/
type Problem struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

/*
ProblemsError carries every problem a rule has.
*/
type ProblemsError struct {
	Problems []Problem
}

/*
Error lists the problems in one line.
*/
func (e *ProblemsError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		parts = append(parts, fmt.Sprintf("%s at %s", p.Code, p.Path))
	}

	return "audience rule invalid: " + strings.Join(parts, ", ")
}

/*
problems wraps found problems in an error, or returns nil when none.
*/
func problems(found ...Problem) error {
	if len(found) == 0 {
		return nil
	}

	return &ProblemsError{Problems: found}
}
