/*
|--------------------------------------------------------------------------
| Tool Catalog: Feedback Boards, Statuses and Links
|--------------------------------------------------------------------------
|
| The entries for the statuses feature requests move through, the boards
| a help center splits them across, and the links between requests and
| the conversations they came up in.
|
*/

package mcpserver

/*
feedbackStatusDocs returns the entries for the statuses feature
requests move through.
*/
func feedbackStatusDocs() map[string]Doc {
	return map[string]Doc{
		"feedback-statuses.index": {
			Title:   "List feature request statuses",
			Use:     "Lists the statuses feature requests move through, in the order the portal and the app show them.",
			Returns: "the statuses with their key, name, color, category and whether the roadmap shows them, in order.",
		},
		"feedback-statuses.update": {
			Title:   "Reorder feature request statuses",
			Use:     "Sets the order the portal and the app show the statuses in.",
			Returns: "the statuses in their new order.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"statuses": "Every status key, such as planned or under_review, once each in the new order. " +
					"Find them with dimmi_feedback_statuses_list.",
			},
		},
		"feedback-statuses.store": {
			Title:   "Add a feature request status",
			Use:     "Adds a status after the others. Its key comes from its name and never changes.",
			Returns: "the status with its key.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"name":     "What readers see, 1 to 40 characters.",
				"color":    "Hex color, such as #3b82f6.",
				"category": "open, planned, in_progress, done or closed. New requests start in the first open status.",
				"roadmap":  "false keeps it off the public roadmap; closed statuses never show there.",
			},
		},
		"feedback-statuses.change": {
			Title:   "Change a feature request status",
			Use:     "Renames, recolors or recategorizes a status, named by its key. Its requests keep it.",
			Returns: "the status as it now is.",
			Risk:    new(RiskReach),
		},
		"feedback-statuses.destroy": {
			Title: "Delete a feature request status",
			Use: "Removes a status, moving its requests to move_to, or else to another status of the same " +
				"category. The last status stays.",
			Params: map[string]string{
				"move_to": "Key of the status its requests move to.",
			},
		},
	}
}

/*
feedbackBoardDocs returns the entries for a help center's boards.
*/
func feedbackBoardDocs() map[string]Doc {
	return map[string]Doc{
		"help-centers.feedback-boards.index": {
			Title:   "List feedback boards",
			Use:     "Lists a help center's boards of feature requests, in the order the portal shows them.",
			Returns: "the boards with visibility, approval and how many requests each holds.",
		},
		"help-centers.feedback-boards.store": {
			Title:   "Add a feedback board",
			Use:     "Adds a board, such as Bugs, after the help center's others.",
			Returns: "the board.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"name":              "The board's name, 1 to 60 characters.",
				"slug":              "Its address on the portal; taken from the name when left out.",
				"visibility":        "public (anyone), private (signed-in customers) or internal (the team only).",
				"approval_required": "true holds what customers post until the team approves it.",
			},
		},
		"help-centers.feedback-boards.update-many": {
			Title:   "Reorder feedback boards",
			Use:     "Sets the order the portal shows a help center's boards in.",
			Returns: "the boards in their new order.",
			Params:  map[string]string{"boards": "Every board id (fbd_...) once, in the new order."},
		},
		"feedback-boards.update": {
			Title:   "Change a feedback board",
			Use:     "Renames a board or changes who sees it and whether it holds posts for approval.",
			Returns: "the board as it now is.",
			Risk:    new(RiskReach),
		},
		"feedback-boards.destroy": {
			Title: "Delete a feedback board",
			Use: "Removes a board, moving its requests onto move_to or the help center's first other board. " +
				"The last board stays.",
			Params: map[string]string{"move_to": "Id of the board (fbd_...) its requests move to."},
		},
	}
}

/*
feedbackLinkDocs returns the entries for linking requests to
conversations.
*/
func feedbackLinkDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.feedback-posts.index": {
			Title:   "List a conversation's feature requests",
			Use:     "Lists the feature requests a conversation was linked to.",
			Returns: "the requests with status and votes.",
		},
		"conversations.feedback-posts.store": {
			Title: "Link a conversation to a feature request",
			Use: "Links a conversation to the request post_id names, or posts a new one from it with a title, " +
				"and votes for the conversation's customer. Search dimmi_help_centers_feedback_posts_list first.",
			Returns: "the request with its new vote count.",
			Risk:    new(RiskWrite),
			Params: map[string]string{
				"post_id":  "Id of the request (fbp_...) to link; leave out to post a new one with title.",
				"title":    "Title of a new request, 1 to 150 characters.",
				"board_id": "With title: the board (fbd_...) to post on; the default board when left out.",
				"priority": "How much it matters to the customer: nice, important or must.",
			},
		},
		"conversations.feedback-posts.destroy": {
			Title: "Unlink a conversation from a feature request",
			Use:   "Takes a conversation's link to a request back. The customer's vote stays.",
		},
		"feedback-posts.conversations.index": {
			Title:   "List a feature request's conversations",
			Use:     "Lists the conversations linked to a request, with each one's customer.",
			Returns: "the conversations with number, subject, status and customer.",
		},
	}
}
