/*
|--------------------------------------------------------------------------
| Tool Catalog: Changelog and Feature Requests
|--------------------------------------------------------------------------
|
| The entries for a help center's changelog, its labels and subscribers,
| and for the feature requests customers post and vote on.
|
*/

package mcpserver

import "maps"

/*
knowledgeDocs2 returns the entries for the changelog and feature
requests.
*/
func knowledgeDocs2() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		changelogDocs1(),
		changelogDocs2(),
		changelogDocs3(),
		feedbackDocs1(),
		feedbackDocs2(),
		feedbackDocs3(),
		feedbackStatusDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
changelogDocs1 returns entries for the changelog, part 1.
*/
func changelogDocs1() map[string]Doc {
	return map[string]Doc{
		"help-centers.changelog-entries.index": {
			Title:   "List changelog entries",
			Use:     "Lists a help center's changelog entries, 25 a page, without their bodies.",
			Returns: "the page of entries with total, and counts of entries in each status.",
			Next:    []string{"dimmi_changelog_entries_get: read one entry's body"},
			Params: map[string]string{
				"status":   "draft, scheduled or published; leave out for all.",
				"label_id": "Id of a label (cll_...) to list only its entries.",
				"query":    "Text to find in titles.",
				"page":     "Numbered page from 1.",
			},
		},
		"help-centers.changelog-entries.store": {
			Title: "Create a changelog draft",
			Use: "Writes a new changelog entry as a draft. Readers and subscribers see nothing until it is " +
				"published with dimmi_changelog_entries_publications_create.",
			Returns: "the draft with its id and lock_version.",
			Next:    []string{"dimmi_changelog_entries_publications_create: publish or schedule it once approved"},
			Params: map[string]string{
				"body_markdown": "The entry in Markdown, at most 256 KB.",
				"summary":       "A short line for lists and mails, at most 300 characters.",
				"cover_url": "An image address from this help center, such as one already used in an article; " +
					"other addresses are refused.",
				"label_ids": "Ids of labels (cll_...), at most 20, from dimmi_help_centers_changelog_labels_list.",
				"author_ids": "Ids of teammates (mem_...) shown as authors, at most 5. Leave out to show whoever " +
					"edits it.",
			},
		},
		"changelog-entries.show": {
			Title: "Get a changelog entry",
			Use: "Reads one changelog entry with the Markdown of its working copy, which may be ahead of the " +
				"live one.",
			Returns: "the entry with body_markdown, status, labels, authors, notify state and lock_version.",
		},
	}
}

/*
changelogDocs2 returns entries for the changelog, part 2.
*/
func changelogDocs2() map[string]Doc {
	return map[string]Doc{
		"changelog-entries.update": {
			Title: "Edit a changelog entry",
			Use: "Saves the entry's working copy. Readers keep seeing the published copy until it is published " +
				"again. Only the fields sent change; label_ids and author_ids replace the whole list.",
			Returns: "the entry as it now is, with its new lock_version.",
			Next:    []string{"dimmi_changelog_entries_publications_create: make the saved changes live"},
			Params: map[string]string{
				"lock_version":  "The lock_version the entry had when you read it. Send it with every save.",
				"body_markdown": "The full new entry in Markdown; it replaces the old text entirely.",
				"cover_url":     "An image address from this help center, such as one already used in an article.",
				"author_ids":    "Ids of teammates (mem_...), at most 5; an empty list shows the editors again.",
			},
			Hint: "changelog_entry_changed means someone saved since you read it: read it again and reapply.",
		},
		"changelog-entries.destroy": {
			Title: "Delete a changelog entry",
			Use:   "Deletes an entry from the team's list and, if it was published, from the public changelog.",
		},
		"changelog-entries.publications.store": {
			Title: "Publish a changelog entry",
			Use: "Makes the entry's working copy live on the public changelog now, or schedules it. With notify " +
				"it is also emailed to every active subscriber, once ever. Confirm the text and whether to email.",
			Returns: "the entry with its new status, published_at or scheduled_for.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"notify": "true emails the entry to active subscribers. It is sent at most once per entry, " +
					"however often it is published again.",
				"published_at":  "RFC 3339 time to date the entry, such as a past ship date; defaults to now.",
				"scheduled_for": "RFC 3339 future time to publish on its own instead of now; unpublished entries only.",
			},
			Hint: "already_published means a live entry cannot be scheduled: unpublish it first, or publish without " +
				"scheduled_for.",
		},
		"changelog-entries.publications.destroy": {
			Title: "Unpublish a changelog entry",
			Use: "Takes an entry off the public changelog, or cancels its schedule, back to a draft. Nothing " +
				"is deleted.",
			Returns: "the entry as a draft.",
			Risk:    new(RiskReach),
		},
		"help-centers.changelog-labels.index": {
			Title: "List changelog labels",
			Use: "Lists the labels a changelog files entries under, such as New, Improved and Fixed, which " +
				"readers can filter by. The defaults are made the first time.",
			Returns: "the labels in order with name, slug and color.",
		},
	}
}

/*
changelogDocs3 returns entries for the changelog, part 3.
*/
func changelogDocs3() map[string]Doc {
	return map[string]Doc{
		"help-centers.changelog-labels.store": {
			Title:   "Create a changelog label",
			Use:     "Adds a label to file entries under. A changelog holds at most 20 labels.",
			Returns: "the new label.",
			Params: map[string]string{
				"name":  "1 to 40 characters, unique in the changelog.",
				"color": "Hex color such as #22c55e.",
			},
		},
		"changelog-labels.update": {
			Title:   "Change a changelog label",
			Use:     "Renames, recolors or reorders a label. Readers see the change on the public changelog at once.",
			Returns: "the label as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"color":    "Hex color such as #22c55e.",
				"position": "Order among the labels, lowest first.",
			},
		},
		"changelog-labels.destroy": {
			Title: "Delete a changelog label",
			Use:   "Deletes a label. Its entries stay, no longer filed under it.",
		},
		"help-centers.changelog-subscribers.index": {
			Title:   "List changelog subscribers",
			Use:     "Lists the people who asked to get changelog entries by email, 25 a page.",
			Returns: "the page of subscribers with email, linked contact, status and total.",
			Params: map[string]string{
				"status": "pending (has not confirmed the email yet), active or unsubscribed; leave out for all.",
				"query":  "Text to find in email addresses.",
				"page":   "Numbered page from 1.",
			},
		},
		"changelog-subscribers.destroy": {
			Title: "Remove a changelog subscriber",
			Use: "Forgets a subscriber, so they get no more changelog mail. Use when someone asks to be removed. " +
				"Find the subscriber with dimmi_help_centers_changelog_subscribers_list.",
		},
	}
}

/*
feedbackDocs1 returns entries for feature requests, part 1.
*/
func feedbackDocs1() map[string]Doc {
	return map[string]Doc{
		"help-centers.feedback-posts.index": {
			Title:   "List feature requests",
			Use:     "Lists a help center's feature requests, 25 a page.",
			Returns: "the page of requests with status, approval, votes and total, and counts in each status.",
			Next:    []string{"dimmi_feedback_posts_get: read one request"},
			Params: map[string]string{
				"status":     "under_review, planned, in_progress, shipped or closed; leave out for all.",
				"moderation": "approved, pending (waiting for the team's approval) or spam.",
				"sort":       "trending, top (most votes) or new.",
				"query":      "Text to find in titles.",
				"page":       "Numbered page from 1.",
			},
		},
		"help-centers.feedback-posts.store": {
			Title: "Post a feature request",
			Use: "Posts a request on the public board for the team, or for a customer who asked for it. It needs " +
				"no approval and is visible at once. Check dimmi_help_centers_feedback_posts_list for a duplicate " +
				"first and vote for that instead.",
			Returns: "the request with its number and public path.",
			Risk:    new(RiskReach),
			Next:    []string{"dimmi_feedback_posts_votes_create: count the customer's vote"},
			Params: map[string]string{
				"body_markdown": "Details in Markdown, at most 64 KB.",
				"contact_id":    "Id of the customer (ctc_...) it is posted for; leave out to post as the team.",
			},
		},
		"feedback-posts.show": {
			Title:   "Get a feature request",
			Use:     "Reads one feature request.",
			Returns: "the request with status, approval, vote and comment counts, and any merge or changelog link.",
			Next: []string{
				"dimmi_feedback_posts_comments_list: read the discussion",
				"dimmi_feedback_posts_votes_list: see who asked for it",
			},
		},
	}
}

/*
feedbackDocs2 returns entries for feature requests, part 2.
*/
func feedbackDocs2() map[string]Doc {
	return map[string]Doc{
		"feedback-posts.update": {
			Title: "Change a feature request",
			Use: "Does one change per call: merge_into_id merges a duplicate, else moderation approves or marks " +
				"spam, else status moves it, else title and body_markdown rewrite it. Changes show on the public " +
				"board at once, and a status change with notify emails every voter and the author.",
			Returns: "the request as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"status": "under_review, planned, in_progress, shipped or closed. Closed leaves the public roadmap.",
				"notify": "With status: true emails the voters and author that it moved. Ask the person first.",
				"message": "With status and notify: the team's note in the email, at most 2000 characters. Write " +
					"it for customers.",
				"changelog_entry_id": "With status: id of the changelog entry (cle_...) that shipped it, " +
					"linked in the email once published.",
				"moderation":    "approved shows a waiting request publicly; spam hides it.",
				"merge_into_id": "Id of the request (fbp_...) to keep. Votes and comments move there.",
			},
		},
		"feedback-posts.destroy": {
			Title: "Delete a feature request",
			Use: "Deletes a request, taking it off the public board. To fold a duplicate into another request, " +
				"merge it with dimmi_feedback_posts_update instead.",
		},
		"feedback-posts.votes.index": {
			Title:   "List a feature request's voters",
			Use:     "Lists the customers who voted for a request, newest first, with how much it matters to them.",
			Returns: "the voters with contact, name, email, company, priority and whether the team cast the vote.",
		},
		"feedback-posts.votes.store": {
			Title: "Vote for a customer",
			Use: "Counts a vote on a request for a customer who asked for it, such as in a conversation. Each " +
				"customer counts once per request.",
			Returns: "the request with its new vote count.",
			Params: map[string]string{
				"contact_id": "Id of the customer (ctc_...) the vote is for.",
				"priority":   "How much it matters to them: nice, important or must.",
			},
		},
		"feedback-posts.votes.destroy": {
			Title:   "Remove a customer's vote",
			Use:     "Takes back a customer's vote on a request.",
			Returns: "the request with its new vote count.",
			Risk:    new(RiskWrite),
		},
	}
}

/*
feedbackDocs3 returns entries for feature requests, part 3.
*/
func feedbackDocs3() map[string]Doc {
	return map[string]Doc{
		"feedback-posts.comments.index": {
			Title:   "List feature request comments",
			Use:     "Lists a request's public comments and the team's internal notes, oldest first.",
			Returns: "the comments with parent, author, internal flag and whether the team wrote them.",
		},
		"feedback-posts.comments.store": {
			Title: "Comment on a feature request",
			Use: "Leaves the team's reply on the public request page, or an internal note only the team sees. " +
				"Replies go one level deep.",
			Returns: "the comment.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"body":      "The comment, 1 to 5000 characters.",
				"internal":  "true keeps it as a note only the team sees; false (default) shows it publicly.",
				"parent_id": "Id of the comment (fbc_...) to reply under.",
			},
		},
		"feedback-comments.destroy": {
			Title: "Delete a feature request comment",
			Use:   "Deletes a comment or internal note on a request.",
		},
	}
}

/*
feedbackStatusDocs returns the entries for the statuses feature
requests move through.
*/
func feedbackStatusDocs() map[string]Doc {
	return map[string]Doc{
		"feedback-statuses.index": {
			Title:   "List feature request statuses",
			Use:     "Lists the statuses feature requests move through, in the order the portal and the app show them.",
			Returns: "the statuses with their key, name and color, in order.",
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
	}
}
