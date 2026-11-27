/*
|--------------------------------------------------------------------------
| Conversation Attribute Step Checks
|--------------------------------------------------------------------------
|
| A step that sets conversation attributes names one to 30 attributes,
| each once, by key, with a value of 2000 characters at most; an empty
| value clears it. Which keys the workspace has, and whether each value
| fits, is for the run to find, so a step stays valid while attributes
| change.
|
*/

package workflowspec

import "unicode/utf8"

/*
attributeChecks returns the checks of the conversation attribute steps.
*/
func (c *checker) attributeChecks() map[string]func(Step) {
	return map[string]func(Step){KindSetConversationAttributes: c.checkSetConversationAttributes}
}

/*
checkSetConversationAttributes checks the values to set.
*/
func (c *checker) checkSetConversationAttributes(s Step) {
	t, ok := decode[SetConversationAttributesSettings](c, s)
	if !ok {
		return
	}

	const field = "attributes"

	switch n := len(t.Attributes); {
	case n == 0:
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: field, Message: "Set at least one attribute."})
	case n > maxTicketValues:
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: field, Message: "Set 30 attributes at most."})
	}

	seen := map[string]bool{}
	for _, v := range t.Attributes {
		if !keyPattern().MatchString(v.Key) || seen[v.Key] {
			c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: field, Message: "Pick each attribute once."})
		}

		seen[v.Key] = true
		if utf8.RuneCountInString(v.Value) > maxTicketValue {
			c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: field, Message: "This value is too long."})
		}

		c.variables(s, field, v.Value)
	}
}
