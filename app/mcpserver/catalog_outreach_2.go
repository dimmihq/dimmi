/*
|--------------------------------------------------------------------------
| Tool Catalog: Sending Broadcasts
|--------------------------------------------------------------------------
|
| The entries that move a broadcast once it is written: send, pause,
| resume, stop and cancel, and the topics people subscribe to. Every one
| changes what customers get, so the person approves each.
|
*/

package mcpserver

/*
broadcastSendDocs returns the entries for sending, pausing, resuming,
stopping and calling off a broadcast.
*/
func broadcastSendDocs() map[string]Doc {
	return map[string]Doc{
		"broadcasts.send.store": {
			Title: "Send a broadcast",
			Use: "Checks everything the broadcast needs, then sends it now or holds it for its schedule. It " +
				"reaches real people and cannot be taken back. Needs the permission to send broadcasts.",
			Returns: "the broadcast with its new status.",
			Next:    []string{"dimmi_broadcasts_report_get: follow the results"},
			Risk:    new(RiskReach),
			Hint: "A missing audience, body, subject or sender is named in the error; fix it with " +
				"dimmi_broadcasts_update.",
		},
		"broadcasts.pause.store": {
			Title:   "Pause a broadcast",
			Use:     "Holds a sending broadcast. Everyone still queued waits until it resumes.",
			Returns: "the broadcast, now paused.",
			Risk:    new(RiskReach),
		},
		"broadcasts.resume.store": {
			Title:   "Resume a broadcast",
			Use:     "Sends a paused broadcast on to everyone still queued.",
			Returns: "the broadcast, sending again.",
			Risk:    new(RiskReach),
		},
		"broadcasts.stop.store": {
			Title: "Stop a broadcast",
			Use: "Ends a sending or paused broadcast for good: everyone still queued is taken off. It cannot " +
				"be resumed.",
			Returns: "the broadcast, now stopped.",
			Risk:    new(RiskDanger),
		},
		"broadcasts.cancel.store": {
			Title:   "Cancel a scheduled broadcast",
			Use:     "Turns a scheduled broadcast back into a draft before it starts, so nobody gets it.",
			Returns: "the broadcast, a draft again.",
			Risk:    new(RiskReach),
			Hint:    "broadcast_status_conflict means it already started: pause or stop it instead.",
		},
	}
}

/*
broadcastTopicDocs returns the entries for the topics people subscribe
to.
*/
func broadcastTopicDocs() map[string]Doc {
	return map[string]Doc{
		"broadcast-topics.index": {
			Title: "List broadcast topics",
			Use: "Lists the topics people subscribe to, such as product updates, so leaving one never means " +
				"leaving all.",
			Returns: "every topic with its consent, description and whether it is archived.",
		},
		"broadcast-topics.store": {
			Title: "Create a broadcast topic",
			Use: "Adds a topic people see on their preference page. An opt_in topic reaches only those who " +
				"said yes; an opt_out topic everyone who has not said no.",
			Returns: "the topic.",
			Risk:    new(RiskReach),
			Params:  topicParams(),
		},
		"broadcast-topics.update": {
			Title: "Change a broadcast topic",
			Use: "Renames, describes or archives a topic. Its consent cannot change once people chose under " +
				"it; archiving keeps every choice made.",
			Returns: "the topic as it now is.",
			Risk:    new(RiskReach),
			Params:  topicParams(),
		},
	}
}

/*
topicParams explains the fields of a topic.
*/
func topicParams() map[string]string {
	return map[string]string{
		"name":        "Name people see, up to 80 characters and unlike the other topics.",
		"description": "What people get under it, up to 300 characters.",
		"consent":     "opt_in or opt_out.",
		"archived":    "true archives it, false brings it back; the choices people made are kept.",
	}
}
