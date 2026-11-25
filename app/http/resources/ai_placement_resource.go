/*
|--------------------------------------------------------------------------
| AI Placement Resources
|--------------------------------------------------------------------------
|
| Somewhere an agent can take new conversations, as the API reads it.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/models"

/*
AIPlacementTarget is somewhere an agent can be placed: a channel
connection, a Slack channel or an inbox. Kind is connection, room or
inbox; Channel is the connection's type, such as widget or email, the
room's kind, or inbox. AgentID is the agent placed there now, null when
the default agent takes it.
*/
type AIPlacementTarget struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Kind    string  `json:"kind"`
	Name    string  `json:"name"`
	Channel string  `json:"channel"`
	AgentID *string `json:"agent_id"`
}

/*
NewAIPlacementTargets shapes the placement targets for the API.
*/
func NewAIPlacementTargets(rows []models.ListPlacementTargetsRow) []AIPlacementTarget {
	prefixes := map[string]models.Prefix{
		"connection": models.PrefixChannelConnection, "room": models.PrefixChannelRoom, "inbox": models.PrefixInbox,
	}
	out := make([]AIPlacementTarget, len(rows))
	for i, r := range rows {
		out[i] = AIPlacementTarget{
			ID: models.PublicID(prefixes[r.Kind], r.ID), Object: "ai_placement_target", Kind: r.Kind, Name: r.Name,
			Channel: r.Channel, AgentID: publicPtr(models.PrefixAiAgent, r.AiAgentID),
		}
	}
	return out
}
