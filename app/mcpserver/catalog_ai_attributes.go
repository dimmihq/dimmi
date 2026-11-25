/*
|--------------------------------------------------------------------------
| Tool Catalog: AI Conversation Attributes
|--------------------------------------------------------------------------
|
| The entries for the facts the AI reads off every conversation, such as
| the customer's plan or how upset they are, and the texts of what they
| answer. Workflows branch and start on them.
|
*/

package mcpserver

/*
aiAttributeDocs returns the entries for AI conversation attributes.
*/
func aiAttributeDocs() map[string]Doc {
	params := map[string]string{
		"key":         "The attribute's key, lowercase letters, digits and underscores, such as order_number.",
		"label":       "What the team calls it, up to 80 characters, such as Order number.",
		"description": "What to look for, for the AI, up to 500 characters.",
		"data_type":   "text, number, date, boolean or choice.",
		"options":     "With choice, the values it may take, at most 30, such as [\"billing\", \"bug\"].",
		"status":      "active to keep reading it, archived to stop.",
	}

	return map[string]Doc{
		"ai.conversation-attributes.index": {
			Title:   "List AI conversation attributes",
			Use:     "Lists the facts the AI reads off each conversation, active and archived.",
			Returns: "the attributes with their key, type, options and status.",
		},
		"ai.conversation-attributes.store": {
			Title: "Add an AI conversation attribute",
			Use: "Adds a fact the AI reads off each conversation and checks again as the customer writes. " +
				"Workflows can start when it changes and branch on its value.",
			Returns: "the attribute.",
			Params:  params,
		},
		"ai.conversation-attributes.update": {
			Title:   "Edit an AI conversation attribute",
			Use:     "Changes an attribute's label, description or options, or archives it. Its key and type stay.",
			Returns: "the attribute as saved.",
			Params:  params,
		},
		"conversations.ai-attributes.index": {
			Title:   "Get a conversation's AI attributes",
			Use:     "Reads what the AI found on a conversation for each active attribute.",
			Returns: "each value found with its attribute, and when the conversation was last read.",
		},
	}
}

/*
aiAttributeSchemaTexts returns the texts of AI conversation attributes.
*/
func aiAttributeSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAttribute": {
			"data_type":   "What the value is: text, number, date, boolean or choice.",
			"description": "What the AI looks for. Empty when not set.",
			"key":         "The attribute's key, as conditions and variables name it: conversation.ai.<key>.",
			"label":       "What the team calls the attribute.",
			"options":     "With choice, the values it may take. Empty otherwise.",
			"status":      "active while the AI reads it, archived once it stopped.",
		},
		"AIAttributeValue": {
			"value": "What the AI found, in the attribute's type: text, a number, a YYYY-MM-DD date, " +
				"true or false, or one of its options.",
		},
		"ConversationAIAttributes": {
			"checked_at": "When the AI last read the conversation. Null when it never has.",
			"values":     "Each value found, with its attribute. An attribute with nothing found is left out.",
		},
	}
}
