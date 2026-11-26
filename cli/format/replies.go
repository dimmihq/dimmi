package format

/*
SavedReply is a canned answer: a Liquid body and what sending it does.
TeamRefs limits it to teams; Unknown lists placeholders the source used
that have no dimmi equivalent, left in the body as written.
*/
type SavedReply struct {
	Base
	Title    string        `json:"title"`
	Body     string        `json:"body"`
	Shortcut string        `json:"shortcut,omitempty"`
	Shared   *bool         `json:"shared,omitempty"`
	OwnerRef string        `json:"owner_ref,omitempty"`
	TeamRefs []string      `json:"team_refs,omitempty"`
	Actions  []ReplyAction `json:"actions,omitempty"`
	Unknown  []string      `json:"unknown_placeholders,omitempty"`
}

/*
ReplyAction is what a saved reply does when sent: add a tag, assign, or
change the status.
*/
type ReplyAction struct {
	Type        string `json:"type"`
	TagRef      string `json:"tag_ref,omitempty"`
	AssigneeRef string `json:"assignee_ref,omitempty"`
	TeamRef     string `json:"team_ref,omitempty"`
	Status      string `json:"status,omitempty"`
}
