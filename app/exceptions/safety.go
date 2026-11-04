/*
|--------------------------------------------------------------------------
| Conversation Safety Errors
|--------------------------------------------------------------------------
|
| What a workspace's own redaction rules and link checks can refuse. Each refusal carries
| a stable code programs can branch on, and never echoes the pattern or
| the text it was given.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Redaction failures.

ErrRedactionRuleLimitReached refuses a rule past the ten a workspace may
keep. ErrInvalidRedactionPattern refuses a pattern that is empty, longer
than 256 characters or not a valid expression. ErrRedactionOrder refuses
an order that does not list each of the workspace's rules exactly once.
*/
var (
	ErrRedactionRuleLimitReached = fault.New(fault.Invalid, "redaction_rule_limit_reached",
		"A workspace keeps at most 10 redaction rules.")
	ErrInvalidRedactionPattern = fault.New(fault.Invalid, "invalid_pattern",
		"The pattern must be a valid expression of at most 256 characters.").OnField("pattern")
	ErrRedactionOrder = fault.New(fault.Invalid, "invalid_order",
		"List each of the workspace's rules exactly once.").OnField("order")
)

/*
ErrInvalidLinkDomains refuses an allowed or blocked domain that is not a
hostname a workspace could own, or a list longer than 500 domains.
*/
var ErrInvalidLinkDomains = fault.New(fault.Invalid, "invalid_link_domains",
	"Each domain must be a hostname such as example.com, at most 500 in a list.").OnField("link_safety")
