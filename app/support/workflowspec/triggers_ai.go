/*
|--------------------------------------------------------------------------
| Conversation Attribute Triggers, Fields And Steps
|--------------------------------------------------------------------------
|
| Teammates, workflows, imports and the AI fill in a conversation's
| attributes. Rules read them as conversation.ai.<key>, whoever set
| them. A workflow can start when any one is set, changed or cleared,
| reading which one, what it became and who set it, or only when the
| AI sets one, as before. A step sets values, which may hold variables.
|
*/

package workflowspec

import "github.com/dimmihq/dimmi/app/support/audience"

/*
The conversation attribute triggers: any change, and the AI's own.
*/
const (
	TriggerAttributeChanged   = "conversation.attribute_changed"
	TriggerAIAttributeChanged = "conversation.ai_attribute_changed"
)

/*
KindSetConversationAttributes sets conversation attribute values.
*/
const KindSetConversationAttributes = "set_conversation_attributes"

/*
AttributeValue sets one conversation attribute, by key, to a value
that may hold variables such as {{run.vars.order_number}}; an empty
value clears it.
*/
type AttributeValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

/*
SetConversationAttributesSettings sets attribute values.
*/
type SetConversationAttributesSettings struct {
	Attributes []AttributeValue `json:"attributes"`
}

/*
aiTriggers returns the triggers of conversation attributes.
*/
func aiTriggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerAttributeChanged, Name: "Conversation attribute changes", Group: GroupDuring,
			Reentry: reentryAny(), Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerAIAttributeChanged, Name: "AI sets a conversation attribute", Group: GroupDuring,
			Reentry: reentryAny(), Default: ReentryEveryTime, ConversationFields: true,
		},
	}
}

/*
attributeKinds returns the conversation attribute step kinds.
*/
func attributeKinds() []Kind {
	return []Kind{{Key: KindSetConversationAttributes, Ready: true, Action: true}}
}

/*
aiFields are the attributes of the conversation, and which one changed,
its new value and who set it under the triggers that say so.
*/
func aiFields() []audience.Field {
	on := func(key string, t audience.FieldType, triggers ...string) audience.Field {
		return audience.Field{
			Key: key, Type: t, Where: audience.Conversation, Trust: audience.Observed, Triggers: triggers,
		}
	}

	return []audience.Field{
		{
			Key: "conversation.ai.", Prefix: true, Type: audience.TypeAuto, Where: audience.Conversation,
			Trust: audience.Observed,
		},
		on("ai_attribute.key", audience.TypeList, TriggerAIAttributeChanged),
		on("ai_attribute.value", audience.TypeAuto, TriggerAIAttributeChanged),
		on("attribute.key", audience.TypeList, TriggerAttributeChanged),
		on("attribute.value", audience.TypeAuto, TriggerAttributeChanged),
		on("attribute.by", audience.TypeList, TriggerAttributeChanged),
	}
}
