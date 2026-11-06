/*
|--------------------------------------------------------------------------
| Agent Message Marks
|--------------------------------------------------------------------------
|
| A reply the agent wrote says so, and shows the sources it was built
| from, so a customer can check an answer and a teammate can open the
| run behind it. Customers see the sources only; the run is for the
| team.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
MessageAI marks a message the agent wrote: the sources it cites and,
for the team, the public id of the run that wrote it.
*/
type MessageAI struct {
	Generated bool              `json:"generated"`
	RunID     *string           `json:"run_id,omitempty"`
	Sources   []MessageAISource `json:"sources"`
}

/*
MessageAISource is one source an answer cites.
*/
type MessageAISource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

/*
storedAI is the agent's mark as the pipeline stores it in a message's
metadata.
*/
type storedAI struct {
	AI *struct {
		RunID     string            `json:"run_id"`
		Generated bool              `json:"generated"`
		Sources   []MessageAISource `json:"sources"`
	} `json:"ai"`
}

/*
teamAIOf reads the agent's mark for the team, nil on a message the
agent did not write.
*/
func teamAIOf(metadata json.RawMessage) *MessageAI {
	out := customerAIOf(metadata)
	if out == nil {
		return nil
	}

	var stored storedAI
	_ = json.Unmarshal(metadata, &stored)
	if id, err := ids.Parse(stored.AI.RunID); err == nil {
		public := models.PublicID(models.PrefixAiRun, id)
		out.RunID = &public
	}
	return out
}

/*
customerAIOf reads the agent's mark as a customer may see it: that the
agent wrote it and what it cites.
*/
func customerAIOf(metadata json.RawMessage) *MessageAI {
	var stored storedAI
	if json.Unmarshal(metadata, &stored) != nil || stored.AI == nil {
		return nil
	}

	return &MessageAI{Generated: true, Sources: orEmpty(stored.AI.Sources)}
}
