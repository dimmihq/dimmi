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
conversationAttributeParams describes the arguments of the conversation
attribute tools.
*/
func conversationAttributeParams() map[string]string {
	return map[string]string{
		"key":   "The attribute's key, lowercase letters, digits and underscores, such as issue_type.",
		"label": "What the team calls it, up to 80 characters, such as Issue type.",
		"description": "What it means, up to 500 characters. The AI reads it to decide the value, so say " +
			"what to look for.",
		"data_type": "text, long_text, number, decimal, choice, multiple_choice, boolean, date, datetime, " +
			"file, contact or company.",
		"options": "With choice or multiple_choice, the values it may take, at most 200.",
		"option_descriptions": "With choice or multiple_choice, what some options mean, by option, 300 " +
			"characters each, such as {\"Billing\": \"Charges, invoices and refunds\"}. The AI reads them.",
		"ids":               "Every attribute's id in the order to show them; those left out follow.",
		"filled_by":         "ai, teammates or both. The AI fills only text, numbers, yes or no, dates and lists.",
		"required_to_close": "true to stop teammates closing a conversation while it is empty.",
		"team_ids":          "The teams that see it in the inbox, every team when empty.",
		"status":            "active to keep it, archived to stop filling it.",
		"values": "The values to set, by key, such as {\"issue_type\": \"Billing\"}; null clears one. " +
			"Each must fit its attribute's type.",
	}
}

/*
conversationAttributeDocs returns the entries for conversation
attributes, which teammates and the AI both fill.
*/
func conversationAttributeDocs() map[string]Doc {
	params := conversationAttributeParams()

	return map[string]Doc{
		"conversation-attributes.index": {
			Title:   "List conversation attributes",
			Use:     "Lists the attributes teammates and the AI fill on every conversation, active and archived.",
			Returns: "the attributes with their key, type, options, who fills them and status.",
		},
		"conversation-attributes.store": {
			Title: "Add a conversation attribute",
			Use: "Adds an attribute every conversation can hold, such as the issue type. Teammates fill it in " +
				"the conversation's details and the AI can fill it as the customer writes.",
			Returns: "the attribute.",
			Params:  params,
		},
		"conversation-attributes.update": {
			Title: "Edit a conversation attribute",
			Use: "Changes an attribute's label, description, options, who fills it, whether it is required " +
				"to close, the teams that see it, or archives it. Its key and type stay.",
			Returns: "the attribute as saved.",
			Params:  params,
		},
		"conversation-attributes.order.update": {
			Name:    "reorder_conversation_attributes",
			Title:   "Reorder conversation attributes",
			Use:     "Sets the order attributes show in, in the inbox's details and in settings.",
			Returns: "the attributes in their new order.",
			Params:  params,
		},
		"conversations.attributes.index": {
			Name:    "get_conversation_attribute_values",
			Title:   "Get a conversation's attributes",
			Use:     "Reads a conversation's attribute values and who set each.",
			Returns: "each active attribute with its value, null when empty, and who set it.",
		},
		"conversations.attributes.update": {
			Name:  "set_conversation_attribute_values",
			Title: "Set a conversation's attributes",
			Use: "Sets or clears some of a conversation's attribute values. A value set here is never " +
				"replaced by the AI.",
			Returns: "the conversation's attributes as they now are.",
			Params:  params,
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
		"ConversationAttribute": {
			"data_type": "What the value is: text, long_text, number, decimal, choice, multiple_choice, " +
				"boolean, date, datetime, file, contact or company.",
			"description": "What it means; the AI reads it to decide the value. Empty when not set.",
			"filled_by":   "Who fills it: ai, teammates or both.",
			"key":         "The attribute's key, as conditions and variables name it: conversation.ai.<key>.",
			"label":       "What the team calls the attribute.",
			"options":     "With choice or multiple_choice, the values it may take. Empty otherwise.",
			"option_descriptions": "What some options mean, by option, for the AI to read. Empty when none " +
				"is described.",
			"required_to_close": "Whether teammates must fill it before closing a conversation.",
			"status":            "active while it is filled, archived once it stopped.",
			"team_ids":          "The teams that see it in the inbox. Empty when every team does.",
		},
		"ConversationAttributeValue": {
			"value": "The value, in the attribute's type: text, a number, true or false, a YYYY-MM-DD date, " +
				"an RFC 3339 date and time, an option, a list of options or file ids, or a contact or company " +
				"id. Null when empty.",
			"set_by": "Who set the value and when. Null when it was never set.",
			"files": "With a file attribute, the files the value holds, each with a short-lived link. " +
				"Empty otherwise.",
		},
		"ConversationAttributeSource": {
			"by":   "ai, member, workflow or import.",
			"id":   "The teammate's id when a teammate set it. Null otherwise.",
			"name": "The teammate's or the workflow's name. Null for the AI and imports.",
			"at":   "When it was set, as an RFC 3339 time.",
			"reason": "Why the AI chose the value, in a sentence. Null when the AI gave none or someone " +
				"else set it.",
			"sureness": "How sure the AI was, from 0 to 1. Null when the AI did not say or someone else set it.",
		},
		"ConversationAttributeValues": {
			"checked_at": "When the AI last read the conversation. Null when it never has.",
			"values":     "Each active attribute with its value, and each archived one that still holds one.",
		},
		"ConversationAIAttributes": {
			"checked_at": "When the AI last read the conversation. Null when it never has.",
			"values":     "Each value found, with its attribute. An attribute with nothing found is left out.",
		},
	}
}
