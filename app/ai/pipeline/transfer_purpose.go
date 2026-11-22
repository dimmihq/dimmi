/*
|--------------------------------------------------------------------------
| Transfer Purposes
|--------------------------------------------------------------------------
|
| An agent that may transfer has to know what each other agent is for,
| not just its name and role, or it answers everything itself. The
| purpose is the opening of the other agent's own guidance, the part
| that says who it talks to, with headings and chips left out.
|
*/

package pipeline

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
purposeLength is about how much of an agent's guidance says what it
is for, in characters.
*/
const purposeLength = 240

/*
agentPurpose is what the agent is for, read from the start of its
guidance, or nothing when it has no guidance.
*/
func agentPurpose(a models.AiAgent) string {
	var kept []string
	size := 0
	for line := range strings.SplitSeq(stripMarkers(a.Guidance), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#-*> "))
		if line == "" || isHeading(line) {
			continue
		}
		kept = append(kept, line)
		size += len(line)
		if size >= purposeLength {
			break
		}
	}
	text := strings.Join(kept, " ")
	if len(text) > purposeLength {
		cut := text[:purposeLength]
		if space := strings.LastIndex(cut, " "); space > 0 {
			cut = cut[:space]
		}
		text = cut + "…"
	}
	return text
}

/*
isHeading reports whether a line is a short title without a sentence's
end, as guidance headings are.
*/
func isHeading(line string) bool {
	return len(line) < 40 && !strings.ContainsAny(line, ".!?:")
}

/*
stripMarkers removes chip markers, which mean nothing outside the
agent's own prompt.
*/
func stripMarkers(text string) string {
	markers := procedure.Markers(text)
	for _, m := range slices.Backward(markers) {
		text = text[:m.Start] + text[m.End:]
	}
	return text
}
