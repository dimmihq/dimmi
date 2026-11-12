/*
|--------------------------------------------------------------------------
| Tool Catalog: Webhooks
|--------------------------------------------------------------------------
|
| The entries for outbound webhooks: the endpoints a workspace posts
| its events to, the events they can subscribe to, their delivery log
| and their signing secrets. A secret lets anyone forge deliveries, so
| reading or replacing one asks the person first; adding or removing
| an endpoint changes where workspace data goes, so it does too.
|
*/

package mcpserver

import "maps"

/*
webhookDocs returns the entries for webhook endpoints and deliveries.
*/
func webhookDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{webhookEndpointDocs(), webhookSecretDocs(), webhookDeliveryDocs()} {
		maps.Copy(out, part)
	}

	return out
}

/*
webhookEndpointDocs returns the entries for listing, adding, changing
and removing endpoints.
*/
func webhookEndpointDocs() map[string]Doc {
	return map[string]Doc{
		"webhook-event-types.index": {
			Title:   "List webhook event types",
			Use:     "Lists every event a webhook endpoint can subscribe to, grouped by area.",
			Returns: "each event type with the object its payload carries, its area and what it means.",
			Next:    []string{"dimmi_webhook_endpoints_create: subscribe an endpoint to some of them"},
		},
		"webhook-endpoints.index": {
			Title:   "List webhook endpoints",
			Use:     "Lists the URLs the workspace posts its events to, with what each subscribes to and its health.",
			Returns: "every endpoint with its URL, event types, whether it is on and its health.",
		},
		"webhook-endpoints.show": {
			Title:   "Get a webhook endpoint",
			Use:     "Reads one endpoint. Its secret is never part of it.",
			Returns: "the endpoint.",
			Next: []string{
				"dimmi_webhook_endpoints_deliveries_list: what was sent to it",
				"dimmi_webhook_endpoints_stats_get: how it answered over the last day",
			},
		},
		"webhook-endpoints.store": {
			Title: "Add a webhook endpoint",
			Use: "Starts posting the workspace's events to a URL. The workspace's owners are told by email. " +
				"Only add URLs the person gave you.",
			Returns: "the endpoint, with its signing secret in meta.secret shown this once.",
			Risk:    new(RiskDanger),
			Next:    []string{"dimmi_webhook_endpoints_tests_create: check it answers"},
			Params:  webhookEndpointParams(),
		},
		"webhook-endpoints.update": {
			Title:   "Change a webhook endpoint",
			Use:     "Changes an endpoint's URL, events, inbox filter or notes, or turns it on or off.",
			Returns: "the endpoint as it now is.",
			Risk:    new(RiskDanger),
			Params:  webhookEndpointParams(),
		},
		"webhook-endpoints.destroy": {
			Title:   "Remove a webhook endpoint",
			Use:     "Stops posting to an endpoint and deletes its delivery log.",
			Returns: "nothing once it is removed.",
			Risk:    new(RiskDanger),
		},
		"webhook-endpoints.stats.show": {
			Title:   "Get webhook endpoint stats",
			Use:     "Reads how an endpoint answered over the last day.",
			Returns: "the attempts, how many succeeded and the median and 95th percentile duration.",
		},
	}
}

/*
webhookSecretDocs returns the entries for an endpoint's secret and test
sends.
*/
func webhookSecretDocs() map[string]Doc {
	return map[string]Doc{
		"webhook-endpoints.secret-reveals.store": {
			Title:   "Reveal a webhook secret",
			Use:     "Reads the secret an endpoint's requests are signed with. Who saw it is recorded.",
			Returns: "the secret and, while a rotation overlaps, the previous one with when it stops.",
			Risk:    new(RiskSensitive),
		},
		"webhook-endpoints.secret-rotations.store": {
			Title: "Rotate a webhook secret",
			Use: "Gives an endpoint a new signing secret. The old one keeps signing for overlap_seconds so the " +
				"receiver can switch without dropping events.",
			Returns: "the new secret and the previous one with when it stops signing.",
			Risk:    new(RiskDanger),
			Params: map[string]string{
				"overlap_seconds": "How long the replaced secret keeps signing, up to a week; 0 stops it at once.",
				"force":           "Rotate even while an earlier rotation still overlaps.",
			},
		},
		"webhook-endpoints.tests.store": {
			Title:   "Send a test webhook",
			Use:     "Posts a sample event to an endpoint now and reports how it answered.",
			Returns: "the test delivery and its attempt with the status, body and a hint on a signature failure.",
			Params: map[string]string{
				"type": "Event type to send a sample of, from dimmi_webhook_event_types_list; " +
					"webhook.test when left out.",
			},
		},
	}
}

/*
webhookDeliveryDocs returns the entries for the delivery log.
*/
func webhookDeliveryDocs() map[string]Doc {
	return map[string]Doc{
		"webhook-endpoints.deliveries.index": {
			Title:   "List webhook deliveries",
			Use:     "Lists the events sent to an endpoint, newest first, and how each went.",
			Returns: "a page of deliveries with their status, attempts and last answer.",
			Next:    []string{"dimmi_webhook_deliveries_get: the payload and every attempt of one"},
			Params: map[string]string{
				"status":     "Only deliveries in this status: pending, retrying, succeeded, failed or canceled.",
				"event_type": "Only deliveries of this event type.",
				"cursor":     "The next_cursor of the previous page.",
				"limit":      "Deliveries per page, up to 100; 25 when left out.",
			},
		},
		"webhook-deliveries.show": {
			Title:   "Get a webhook delivery",
			Use:     "Reads one delivery with the exact payload it sends and every attempt.",
			Returns: "the delivery, its payload and its attempts with their requests and answers.",
			Next:    []string{"dimmi_webhook_deliveries_redeliveries_create: send it again"},
		},
		"webhook-deliveries.redeliveries.store": {
			Title:   "Send a webhook delivery again",
			Use:     "Posts a delivery to its endpoint again now, with the same payload signed afresh.",
			Returns: "the delivery with the new attempt.",
		},
	}
}

/*
webhookEndpointParams explains the fields of an endpoint.
*/
func webhookEndpointParams() map[string]string {
	return map[string]string{
		"url":         "The https URL to post to.",
		"description": "A note for the team, up to 255 characters.",
		"event_types": "Event types from dimmi_webhook_event_types_list, an area such as conversation.*, or * " +
			"for every event.",
		"include_notes": "Also send events about internal notes.",
		"inbox_ids":     "Only conversations of these inboxes (inbox_...); an empty list means every inbox.",
		"api_version":   "Payload version; only the current one is accepted.",
		"metadata":      "Up to 20 short labels for the developer's own use.",
		"enabled":       "false stops sending and cancels what is waiting; true starts again.",
	}
}
