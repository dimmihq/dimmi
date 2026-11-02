/*
|--------------------------------------------------------------------------
| Validation Errors
|--------------------------------------------------------------------------
|
| A failed validation is data, not text: each broken rule keeps the path
| of its field, the rule's code and a message safe to show a person. The
| HTTP layer turns it into the 422 body, the field paths becoming the keys
| of "errors", and other transports read the same violations.
|
*/

package validation

import (
	"fmt"
	"strings"
)

/*
Violation is one broken rule.

Field is the dotted path of the field, such as "emails.0" for the first
element of the emails list. Rule is the code of the rule, such as
"required" or "email", and Param its parameter, such as "255" for max.
Message describes the problem to a person.
*/
type Violation struct {
	Field   string
	Rule    string
	Param   string
	Message string
}

/*
Error is a failed validation, holding every broken rule in field order.
*/
type Error struct {
	Violations []Violation
}

/*
Error lists every field with its message, for logs.
*/
func (e *Error) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		parts = append(parts, v.Field+": "+v.Message)
	}

	return "validation: " + strings.Join(parts, "; ")
}

/*
Message summarizes the failure in one sentence: the first message,
followed by how many more there are.
*/
func (e *Error) Message() string {
	if len(e.Violations) == 0 {
		return ""
	}

	first := e.Violations[0].Message

	switch more := len(e.Violations) - 1; more {
	case 0:
		return first
	case 1:
		return first + " (and 1 more error)"
	default:
		return fmt.Sprintf("%s (and %d more errors)", first, more)
	}
}

/*
Messages returns the messages of each field, keyed by field path.
*/
func (e *Error) Messages() map[string][]string {
	messages := make(map[string][]string, len(e.Violations))
	for _, v := range e.Violations {
		messages[v.Field] = append(messages[v.Field], v.Message)
	}

	return messages
}
