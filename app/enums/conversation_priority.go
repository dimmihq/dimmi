/*
|--------------------------------------------------------------------------
| Conversation Priority
|--------------------------------------------------------------------------
|
| Five levels from none to urgent. The database keeps a small number so
| that sorting is plain index order; only this file knows the mapping,
| and the number never leaves the server.
|
*/

package enums

/*
ConversationPriority is how urgent a conversation is, as the API names
it.
*/
type ConversationPriority string

/*
The priority levels, lowest first.
*/
const (
	ConversationPriorityNone   ConversationPriority = "none"
	ConversationPriorityLow    ConversationPriority = "low"
	ConversationPriorityMedium ConversationPriority = "medium"
	ConversationPriorityHigh   ConversationPriority = "high"
	ConversationPriorityUrgent ConversationPriority = "urgent"
)

/*
ConversationPriorities returns every level, lowest first, each at the
position of the number the database stores for it.
*/
func ConversationPriorities() []ConversationPriority {
	return []ConversationPriority{
		ConversationPriorityNone, ConversationPriorityLow, ConversationPriorityMedium,
		ConversationPriorityHigh, ConversationPriorityUrgent,
	}
}

/*
Level returns the number the database stores for the priority, and
false when the priority is not one of the levels.
*/
func (p ConversationPriority) Level() (int16, bool) {
	for level, known := range ConversationPriorities() {
		if known == p {
			return int16(level), true
		}
	}

	return 0, false
}

/*
ConversationPriorityOf returns the priority the stored number means. A
number outside the range reads as none.
*/
func ConversationPriorityOf(level int16) ConversationPriority {
	all := ConversationPriorities()
	if level < 0 || int(level) >= len(all) {
		return ConversationPriorityNone
	}

	return all[level]
}
