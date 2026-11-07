/*
|--------------------------------------------------------------------------
| Action Step Settings
|--------------------------------------------------------------------------
|
| The typed settings of more step kinds; see the step settings file for
| how ids and optional switches are written.
|
*/

package workflowspec

/*
UnassignSettings clears the member, the team or both.
*/
type UnassignSettings struct {
	What string `json:"what"`
}

/*
TagSettings adds or removes tags.
*/
type TagSettings struct {
	TagIDs []string `json:"tag_ids"`
}

/*
PrioritySettings sets the conversation's priority.
*/
type PrioritySettings struct {
	Value string `json:"value"`
}

/*
NoteSettings leaves an internal note, optionally mentioning members.
*/
type NoteSettings struct {
	Text             string   `json:"text"`
	MentionMemberIDs []string `json:"mention_member_ids,omitempty"`
}

/*
ContactChange sets one contact field to a value with variables.
*/
type ContactChange struct {
	Field string `json:"field"`
	Value string `json:"value"`
}
