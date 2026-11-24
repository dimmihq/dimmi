/*
|--------------------------------------------------------------------------
| The Messenger API's Words
|--------------------------------------------------------------------------
|
| The messenger API is what the messenger and its SDKs call for a
| customer. It is not offered as tools, but its document is explained
| the same way: every operation, parameter and field says what it is.
| Records it shares with the REST API keep the words written for them
| there; the ones of its own are written here, by schema and field.
|
*/

package mcpserver

/*
messengerSchemaTexts returns the texts of the messenger API's schemas:
those written for the REST API's records of the same name, with the
messenger's own laid over them.
*/
func messengerSchemaTexts() map[string]map[string]string {
	out := schemaTexts()
	for _, part := range messengerTextParts() {
		mergeTexts(out, part)
	}

	mergeTexts(out, pollTexts())

	return out
}

/*
pollTexts explains what the answer to a poll for updates says beside
the changes themselves.
*/
func pollTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"MetaStruct": {
			"cursor": "Where this answer ends. Send it as since on the next poll to get only what changed " +
				"after it.",
			"has_more": "true when more changes wait than this answer holds: poll again at once with the " +
				"cursor. false when you are caught up.",
			"poll_after_ms": "How many milliseconds to wait before the next poll, such as 5000. 0 when " +
				"has_more is true, since you should poll again at once.",
			"server_time": "The server's time when it answered, as an RFC 3339 time in UTC, such as " +
				"2026-01-31T09:00:00Z.",
		},
	}
}
