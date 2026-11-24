/*
|--------------------------------------------------------------------------
| AI Attribute Triggers And Fields
|--------------------------------------------------------------------------
|
| The decision model fills in a workspace's AI conversation attributes
| and checks them again as the customer writes. Rules read them as
| conversation.ai.<key>, and a workflow can start when one is set or
| changes, reading which one and what it became.
|
*/

package workflowspec

import "github.com/dimmihq/dimmi/app/support/audience"

/*
TriggerAIAttributeChanged starts a workflow when the decision model
sets or changes an AI attribute of a conversation.
*/
const TriggerAIAttributeChanged = "conversation.ai_attribute_changed"

/*
aiTriggers returns the triggers of AI attributes.
*/
func aiTriggers() []Trigger {
	return []Trigger{{
		Key: TriggerAIAttributeChanged, Name: "AI attribute changes", Group: GroupDuring, Reentry: reentryAny(),
		Default: ReentryEveryTime, ConversationFields: true,
	}}
}

/*
aiFields are the AI attributes of the conversation, and which one
changed and its new value under the trigger that says so.
*/
func aiFields() []audience.Field {
	on := func(key string, t audience.FieldType) audience.Field {
		return audience.Field{
			Key: key, Type: t, Where: audience.Conversation, Trust: audience.Observed,
			Triggers: []string{TriggerAIAttributeChanged},
		}
	}

	return []audience.Field{
		{
			Key: "conversation.ai.", Prefix: true, Type: audience.TypeAuto, Where: audience.Conversation,
			Trust: audience.Observed,
		},
		on("ai_attribute.key", audience.TypeList),
		on("ai_attribute.value", audience.TypeAuto),
	}
}
