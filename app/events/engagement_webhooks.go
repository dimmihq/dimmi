/*
|--------------------------------------------------------------------------
| Engagement Webhooks
|--------------------------------------------------------------------------
|
| The subscribable events of what the team publishes and sends: the
| changelog, feature requests, broadcasts and series. Each area is its
| own group in the dashboard's picker and its own wildcard.
|
*/

package events

import "slices"

/*
engagementWebhooks lists the events about the changelog, feedback,
broadcasts and series.
*/
func engagementWebhooks() []WebhookEventType {
	return slices.Concat(changelogWebhooks(), feedbackWebhooks(), sendingWebhooks(), ticketWebhooks())
}

/*
changelogWebhooks lists the events about changelog entries.
*/
func changelogWebhooks() []WebhookEventType {
	c := func(name, description string) WebhookEventType {
		return WebhookEventType{
			Type: "changelog_entry." + name, Object: "changelog_entry", Group: "Changelog", Description: description,
		}
	}

	return []WebhookEventType{
		c("published", "A changelog entry went live."),
		c("updated", "A live changelog entry was published again with edits."),
		c("unpublished", "A changelog entry was taken back to a draft."),
	}
}

/*
feedbackWebhooks lists the events about feature requests.
*/
func feedbackWebhooks() []WebhookEventType {
	f := func(name, description string) WebhookEventType {
		return WebhookEventType{
			Type: "feedback_post." + name, Object: "feedback_post", Group: "Feedback", Description: description,
		}
	}

	return []WebhookEventType{
		f("created", "A feature request was posted."),
		f("status_changed", "A feature request moved to another status."),
		f("voted", "A customer's vote for a feature request was counted."),
		f("merged", "A duplicate feature request was merged into this one."),
		f("linked_conversation", "A conversation was linked to a feature request."),
	}
}

/*
sendingWebhooks lists the events about broadcasts and series.
*/
func sendingWebhooks() []WebhookEventType {
	return []WebhookEventType{
		{
			Type: "broadcast.sent", Object: "broadcast", Group: "Broadcasts",
			Description: "A broadcast finished sending.",
		},
		{
			Type: "broadcast.paused", Object: "broadcast", Group: "Broadcasts",
			Description: "A sending broadcast was paused, by a teammate or for bounces or complaints.",
		},
		{
			Type: "series.published", Object: "series", Group: "Series",
			Description: "A series went live with a new version.",
		},
		{
			Type: "series.journey_completed", Object: "series_enrollment", Group: "Series",
			Description: "A person's journey through a series ended: finished, reached the goal or left.",
		},
	}
}
