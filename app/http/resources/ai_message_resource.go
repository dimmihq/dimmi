/*
|--------------------------------------------------------------------------
| Agent Message Marks
|--------------------------------------------------------------------------
|
| A reply the agent wrote says so, says how it ended, and shows the
| sources it was built from, so a customer can check an answer and a
| teammate can open the run behind it. Customers see the outcome, the
| sources, the confirmation card of a change the agent proposes and
| where that change stands; the run, the arguments and who decided are
| for the team.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
MessageAI marks a message the agent wrote: how its run ended, such as
answered or handoff, so the messenger offers the right follow-ups; the
sources it cites; and, for the team, the public id of the run that
wrote it. Confirmation is the card of a change waiting for the
customer's yes; ActionCall is the change the reply reports and where it
stands, its arguments, decider and result for the team only.
*/
type MessageAI struct {
	Generated    bool                   `json:"generated"`
	Outcome      string                 `json:"outcome,omitempty"`
	RunID        *string                `json:"run_id,omitempty"`
	Sources      []MessageAISource      `json:"sources"`
	Confirmation *MessageAIConfirmation `json:"confirmation,omitempty"`
	ActionCall   *MessageAIActionCall   `json:"action_call,omitempty"`
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
		RunID        string                 `json:"run_id"`
		Generated    bool                   `json:"generated"`
		Outcome      string                 `json:"outcome"`
		Sources      []MessageAISource      `json:"sources"`
		Confirmation *MessageAIConfirmation `json:"confirmation"`
		ActionCall   *MessageAIActionCall   `json:"action_call"`
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
	out.ActionCall = stored.AI.ActionCall
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

	out := &MessageAI{
		Generated: true, Outcome: stored.AI.Outcome, Sources: orEmpty(stored.AI.Sources),
		Confirmation: stored.AI.Confirmation,
	}
	if c := stored.AI.Confirmation; c != nil {
		c.Details = orEmpty(c.Details)
	}
	if c := stored.AI.ActionCall; c != nil {
		out.ActionCall = &MessageAIActionCall{ID: c.ID, State: c.State, Summary: c.Summary}
	}
	return out
}
