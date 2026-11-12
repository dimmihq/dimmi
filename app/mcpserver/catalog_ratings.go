/*
|--------------------------------------------------------------------------
| Tool Catalog: Conversation Ratings
|--------------------------------------------------------------------------
|
| The entries for how customers rated conversations and for the
| workspace's settings on asking them.
|
*/

package mcpserver

/*
ratingDocs returns the entries for conversation ratings and their
settings.
*/
func ratingDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.ratings.index": {
			Title: "List a conversation's ratings",
			Use: "Reads how the customer rated a closed conversation: each answered ask, newest first, with its " +
				"score from 1 (terrible) to 5 (amazing), its face and word, the comment, the channel it was asked " +
				"on and the teammate it rated.",
			Returns: "the ratings, newest first; empty when the customer never answered.",
		},
		"conversation-rating-settings.show": {
			Title: "Read rating settings",
			Use: "Reads whether and how the workspace asks customers to rate a conversation " +
				"once it closes.",
			Returns: "whether asking is on, the delay in minutes, whether agent-closed conversations ask, " +
				"and the channels.",
			Next: []string{"dimmi_conversation_rating_settings_update: change how ratings are asked"},
		},
		"conversation-rating-settings.update": {
			Title: "Update rating settings",
			Use: "Turns asking customers for a rating on close on or off and changes how: the delay after the " +
				"close, whether conversations the agent closed ask too, and which channels ask. Customers see the " +
				"ask in the messenger or by email from then on.",
			Returns: "the settings as saved.",
			Params: map[string]string{
				"enabled":              "true asks customers to rate conversations when they close.",
				"delay_minutes":        "0 asks straight away, 10 after ten minutes, 60 after an hour.",
				"include_agent_closed": "true also asks when the AI agent closed the conversation.",
				"messenger":            "true asks in the messenger.",
				"email":                "true asks email conversations' customers by email.",
			},
		},
	}
}
