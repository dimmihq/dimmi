/*
|--------------------------------------------------------------------------
| What A Decision Reads
|--------------------------------------------------------------------------
|
| A decision about a conversation reads all of it: the subject, then
| every public message marked by who wrote it, the customer, the team,
| an agent or the automation. Notes and timeline events stay out: they
| are the team's own talk, not the conversation. Long conversations keep
| their newest words.
|
*/

package aidecision

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Transcript writes the conversation, oldest message first, as the
models read it.
*/
func Transcript(subject *string, msgs []models.Message) string {
	var lines []string

	if subject != nil && strings.TrimSpace(*subject) != "" {
		lines = append(lines, "Subject: "+strings.TrimSpace(*subject))
	}

	for _, m := range msgs {
		if said(m) {
			lines = append(lines, speaker(m.AuthorType)+": "+strings.TrimSpace(*m.Body))
		}
	}

	return newest(strings.Join(lines, "\n\n"))
}

/*
LastCustomerMessage is the customer's newest message alone, from
messages oldest first.
*/
func LastCustomerMessage(msgs []models.Message) string {
	for _, m := range slices.Backward(msgs) {
		if said(m) && m.AuthorType == string(enums.AuthorTypeContact) {
			return newest(strings.TrimSpace(*m.Body))
		}
	}

	return ""
}

/*
said reports whether a message is a public message with words in it.
*/
func said(m models.Message) bool {
	return m.Type == string(enums.MessageTypeMessage) && m.Body != nil && strings.TrimSpace(*m.Body) != ""
}

/*
speaker names who wrote a message.
*/
func speaker(author string) string {
	switch enums.AuthorType(author) {
	case enums.AuthorTypeContact:
		return "Customer"
	case enums.AuthorTypeMember:
		return "Team"
	case enums.AuthorTypeAgent:
		return "AI agent"
	case enums.AuthorTypeSystem, enums.AuthorTypeIntegration:
	}

	return "Automation"
}
