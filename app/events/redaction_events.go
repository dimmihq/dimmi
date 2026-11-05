/*
|--------------------------------------------------------------------------
| Redaction Events
|--------------------------------------------------------------------------
|
| A workspace's redaction changed: a rule was added, changed or removed,
| or the built-in rules were switched or the rules reordered. These stay
| inside the application, where the audit log records them; no webhook
| and no open screen hears them. A message that lost text to a rule says
| which rules and how often, never the text.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
RedactionRuleCreated is dispatched when a workspace adds a rule. Name is
the rule's name.
*/
type RedactionRuleCreated struct {
	Base

	RuleID ids.UUID
	Name   string
}

/*
EventName names the event.
*/
func (RedactionRuleCreated) EventName() string { return "redaction_rule.created" }

/*
RedactionRuleUpdated is dispatched when a rule changes. Changes lists
the fields that changed.
*/
type RedactionRuleUpdated struct {
	Base

	RuleID  ids.UUID
	Name    string
	Changes []string
}

/*
EventName names the event.
*/
func (RedactionRuleUpdated) EventName() string { return "redaction_rule.updated" }

/*
RedactionRuleDeleted is dispatched when a rule is removed.
*/
type RedactionRuleDeleted struct {
	Base

	RuleID ids.UUID
	Name   string
}

/*
EventName names the event.
*/
func (RedactionRuleDeleted) EventName() string { return "redaction_rule.deleted" }

/*
RedactionSettingsUpdated is dispatched when the built-in rules are
switched or the rules reordered. Changes maps each switch that changed
to its old and new value, and holds order when the rules moved.
*/
type RedactionSettingsUpdated struct {
	Base

	Changes map[string]any
}

/*
EventName names the event.
*/
func (RedactionSettingsUpdated) EventName() string { return "redaction_settings.updated" }

/*
MessageRedacted is dispatched when rules replaced text in a message
before it was stored. Rules names each rule that hit and Count is how
many matches they replaced in all. A channel that keeps the message's
raw source drops it on hearing this.
*/
type MessageRedacted struct {
	Base
	MessageRef

	Rules []string
	Count int
}

/*
EventName names the event.
*/
func (MessageRedacted) EventName() string { return "message.redacted" }
