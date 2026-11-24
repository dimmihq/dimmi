/*
|--------------------------------------------------------------------------
| The Messenger API's Words: WidgetTicketType To WidgetUploadTarget
|--------------------------------------------------------------------------
|
| What each field of these messenger records means, by schema and field.
|
*/

package mcpserver

/*
messengerTextsFromWidgetTicketType returns the texts from WidgetTicketType on.
*/
func messengerTextsFromWidgetTicketType() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetTicketType": {
			"name": "The type's name as shown to the customer, such as Bug report.",
			"states": "The states a request of this type moves through, with their customer-facing " +
				"labels. Empty when the type appears inside a request.",
		},
		"WidgetTrackedEvent": {
			"status": "What became of the event: accepted, duplicate (its id was already sent), " +
				"contact_not_found, archived, event_limit_reached or not_allowed.",
		},
		"WidgetTrackedEvents": {
			"accepted": "How many events of the batch were accepted.",
			"events":   "One result per event sent, in the same order as the request.",
		},
		"WidgetUpload": {
			"links": "Related addresses keyed by name. self is this upload's own address, which you " +
				"PATCH with status uploaded once the bytes are sent.",
			"status": "The upload's state. Always pending here, until you send the bytes and mark it " +
				"uploaded.",
		},
		"WidgetUploadTarget": {
			"expires_at": "When the upload address stops working, as an RFC 3339 timestamp. Ask for a " +
				"new upload after that.",
			"headers": "The headers to send with the upload exactly as given: Content-Type and " +
				"Content-Length, matching what was declared.",
			"method": "The HTTP method to send the file's bytes with. Always PUT.",
			"url": "The address to send the file's bytes to: signed storage that takes them as they " +
				"are, or dimmi's own upload route, which also needs the session's bearer token.",
		},
	}
}
